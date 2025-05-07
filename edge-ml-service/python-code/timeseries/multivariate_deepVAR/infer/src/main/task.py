import os
import pandas as pd
import uvicorn
import numpy as np
import traceback
from typing import Dict, Any
from gluonts.dataset.common import ListDataset
from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.ml.hedge_inference import HedgeInferenceBase, Inputs
from common.src.ml.hedge_api import InferenceAPI
from gluonts.dataset.field_names import FieldName
from common.src.util.infer_exception import HedgeInferenceException

class TimeseriesInputs(Inputs):
    __root__: Dict[str, Any]

class TimeseriesDeepVARInference(HedgeInferenceBase):
    def __init__(self):
        self.logger = LoggerUtil().logger
        self.env_util = EnvironmentUtil(os.getcwd() + '/timeseries/multivariate_deepVAR/env.yaml')

        super().get_env_vars()
        self.port = self.env_util.get_env_value('PORT', self.env_util.get_env_value('Service.port', '55000'))

        self.artifacts_dict = {"models": "predictor.gz",
                               "params": "model_params_dict.gz",
                               "config": "assets/config.json"}

        super().clear_and_initialize_model_dict(self.artifacts_dict.keys())

    def read_model_config(self):
        pass

    def validate_confidence_interval(self, confidence_interval):
        """
        Validates that the confidence interval min and max values meet the required conditions.

        Args:
            confidence_interval (dict): A dictionary containing 'min' and 'max' keys.

        Raises:
            ValueError: If any of the conditions are not met.
        """
        quantile_min = confidence_interval.get("min")
        quantile_max = confidence_interval.get("max")

        if quantile_min is None or quantile_max is None:
            raise ValueError("Both 'min' and 'max' must be provided in the confidence interval.")

        # Check if they are valid floats
        if not isinstance(quantile_min, (float, int)) or not isinstance(quantile_max, (float, int)):
            raise ValueError("'min' and 'max' must be valid numeric values (float or int).")

        # Convert to float explicitly (handles int inputs gracefully)
        quantile_min = float(quantile_min)
        quantile_max = float(quantile_max)

        if not (0.0 < quantile_min < 1.0):
            raise ValueError(f"'min' value ({quantile_min}) must be between 0.0 and 1.0, exclusive.")

        if not (0.0 < quantile_max < 1.0):
            raise ValueError(f"'max' value ({quantile_max}) must be between 0.0 and 1.0, exclusive.")

        if quantile_min >= quantile_max:
            raise ValueError(f"'min' value ({quantile_min}) must be less than 'max' value ({quantile_max}).")


    def predict(self, ml_algorithm: str, training_config: str, external_input: TimeseriesInputs):
        try:
            if not self.model_dict["models"]:
                raise RuntimeError("Models have not been loaded. Please initialize the models.")

            data_dict = external_input.__root__
            correlation_ids = list(data_dict.keys())
            self.logger.info(f"Correlation IDs: {correlation_ids}")

            if len(correlation_ids) == 0:
                raise HedgeInferenceException(status_code=400, detail="Correlation IDs list is empty")

            ml_algorithm_dict = self.model_dict["models"].get(ml_algorithm, {})

            predictor = ml_algorithm_dict.get(training_config, None)
            ml_params_dict = self.model_dict["params"].get(ml_algorithm, {})
            params = ml_params_dict.get(training_config, None)

            if predictor is None or params is None:
                raise HedgeInferenceException(status_code=404, detail="No model found")

            target_columns = params["target_columns"]
            group_by_cols = params["group_by_cols"]
            freq = params["freq"]
            date_field = params["date_field"]

            # The below lists will maintain the order using `featureNameToColumnIndex`
            input_columns = params["input_columns"]
            output_columns = params["output_columns"]

            test_list = []
            for correlation_id in correlation_ids:
                input_data = data_dict[correlation_id]["data"]

                # Validate confidence interval again after setting default values, in case of previous validation failure
                if "confidence_interval" in data_dict[correlation_id] and data_dict[correlation_id]["confidence_interval"]:
                    self.logger.info(f"Confidence interval found for correlation ID: {correlation_id}")
                    self.validate_confidence_interval(data_dict[correlation_id]["confidence_interval"])
                else:
                    self.logger.info(f"No confidence interval found for correlation ID: {correlation_id}")
                    data_dict[correlation_id]["confidence_interval"] = {
                        "min": 0.25,
                        "max": 0.75
                    }
                    self.logger.info(f"Default confidence interval set: {data_dict[correlation_id]['confidence_interval']}")


                inputs_df = pd.DataFrame(input_data, columns=input_columns)

                grouped_data = inputs_df.groupby(group_by_cols)

                for group_key, group_df in grouped_data:
                    # Sort group data by timestamp
                    group_df = group_df.sort_values(date_field)

                    test_target = group_df[target_columns].values.T
                    test_start_time = pd.to_datetime(group_df[date_field].iloc[0], unit="s")
                    self.logger.info(f'{test_start_time=}')
                    test_item_id = "_".join(map(str, group_key))

                    test_list.append({
                        FieldName.TARGET: test_target,
                        FieldName.START: test_start_time,
                        FieldName.ITEM_ID: test_item_id
                    })

            # Call predict method only once for different multi-timeseries
            predict_ds = ListDataset(data_iter=test_list, freq=freq, one_dim_target=False)
            forecasts = predictor.predict(predict_ds)

            results = {}
            for forecast in forecasts:
                predictions, min_boundaries, max_boundaries = [], [], []

                self.logger.info(f'Item_ID: {forecast.item_id}')
                self.logger.info(f'Predictions Start Datetime: {forecast.start_date.to_timestamp()}')
                self.logger.info(f'Predictions Count: {len(forecast.mean)}')
                self.logger.info(f'Frequency: {forecast.freq=}')

                confidence_interval = data_dict[forecast.item_id]["confidence_interval"]

                item_id = forecast.item_id
                group_keys = item_id.split('_') if len(group_by_cols) > 1 else [item_id]

                # Generate timestamps for the forecast period
                timestamps = pd.date_range(
                    start=forecast.start_date.to_timestamp(),
                    periods=len(forecast.median),
                    freq=forecast.freq
                ).astype('int64') // 10**9

                # Calculate quantiles for min/max boundaries
                self.logger.info(f"{confidence_interval['min']=} and {confidence_interval['max']=}")
                min_quantiles = np.round(forecast.quantile(confidence_interval['min']), 2)
                max_quantiles = np.round(forecast.quantile(confidence_interval['max']), 2)

                self.logger.info(f'{min_quantiles=}')

                # Append group key, timestamps, and values to predictions, min_boundary, and max_boundary
                for i, ts in enumerate(timestamps):
                    predictions.append(
                        [ts] + group_keys + [num for num in forecast.median[i].tolist()]
                    )

                    min_boundaries.append(
                        [ts] + group_keys + [num for num in min_quantiles[i]]
                    )

                    max_boundaries.append(
                        [ts] + group_keys + [num for num in max_quantiles[i]]
                    )

                # Preprocess predictions, min_boundaries, and max_boundaries
                predictions_rounded = (
                    pd.DataFrame(predictions, columns=input_columns)[output_columns]
                    .applymap(lambda x: round(x, 2) if isinstance(x, float) else x)
                    .values.tolist()
                )

                min_boundaries_rounded = (
                    pd.DataFrame(min_boundaries, columns=input_columns)[output_columns]
                    .applymap(lambda x: round(x, 2) if isinstance(x, float) else x)
                    .values.tolist()
                )

                max_boundaries_rounded = (
                    pd.DataFrame(max_boundaries, columns=input_columns)[output_columns]
                    .applymap(lambda x: round(x, 2) if isinstance(x, float) else x)
                    .values.tolist()
                )

                # Populate results
                results[item_id] = {
                    "predictions": predictions_rounded,
                    "min_boundary": min_boundaries_rounded,
                    "max_boundary": max_boundaries_rounded,
                }
            self.logger.info(f"Prediction results: {results}")
            return results
        except RuntimeError as e:
            self.logger.error(f"Error while loading models: {e}")
            raise HedgeInferenceException(status_code=500, detail=f'Error while loading models: {str(e)}')
        except Exception as e:
            self.logger.error(f"Prediction failed  with error: {e}")
            self.logger.error(traceback.format_exc())
            if 'status_code' in e.__dict__:
                raise
            else:
                raise HedgeInferenceException(status_code=500, detail=f'Prediction failed with error: {str(e)}')

if __name__ == "__main__":

    ts_inputs = TimeseriesInputs(__root__={
        "dev_1": {
            "data": [
                [1672617600.0,  "dev_1", 2000.0, 10.0, 150.0, 1.001, 20.5],
                [1672621200.0,  "dev_1", 2050.0, 11.0, 155.0, 1.002, 21.0]
            ],
            "confidence_interval": {"min": 0.6, "max": 0.8},
        },
        "dev_2": {
            "data": [
                [1672617600.0, "dev_2", 1800.0, 9.5, 140.0, 1.000, 19.8],
                [1672621200.0, "dev_2", 1850.0, 10.0, 145.0, 1.001, 20.0]
            ],
            "confidence_interval": {"min": 0.65, "max": 0.85},
        }
    })

    try:
        inference_obj = TimeseriesDeepVARInference()
        inference_obj.load_model(inference_obj.model_dir, inference_obj.artifacts_dict)
    except Exception as e:
        raise Exception(f"Unexpected error while loading models: {e}")

    try:
        app = InferenceAPI(inference_obj, inputs=ts_inputs, outputs=None)
        uvicorn.run(app, host=inference_obj.host, port=int(inference_obj.port))
    except Exception as e:
        raise Exception(f"Unexpected error while starting the Inference container: {e}")
