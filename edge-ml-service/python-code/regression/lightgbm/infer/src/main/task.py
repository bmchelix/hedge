"""
(c) Copyright 2020-2025 BMC Software, Inc.
Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import os
import pandas as pd
import uvicorn
import traceback
from typing import List, Union, Dict

from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.ml.hedge_inference import HedgeInferenceBase, Inputs, Outputs
from common.src.util.config_extractor import FeatureExtractor
from common.src.ml.hedge_api import InferenceAPI
from common.src.ml import start_fs_monitor


class RegressionInputs(Inputs):
    """Defines the input payload for LightGBM Regression."""
    __root__: dict[str, List[Union[float, str]]]


class RegressionOutputs(Outputs):
    """Defines the output payload for LightGBM Regression."""
    __root__: Dict[str, float]


class LightGBMRegressionInference(HedgeInferenceBase):
    """
    This class handles the inference process for the LightGBM regression model.
    It includes model loading, data preprocessing, prediction, and output formatting.
    """
    model_dir = None
    logger = None
    model_dict = None
    port = None
    host = None

    def __init__(self):
        self.logger = LoggerUtil().logger
        self.env_util = EnvironmentUtil(os.getcwd() + '/regression/lightgbm/env.yaml')

        super().get_env_vars()
        self.port = self.env_util.get_env_value('PORT', self.env_util.get_env_value('Service.port', '51000'))

        self.artifacts_dict = {
            "models": "model.gz",
            "artifacts": "artifacts.gz",
            "config": "assets/config.json"
        }

        super().clear_and_initialize_model_dict(self.artifacts_dict.keys())

    def read_model_config(self):
        pass

    def predict(self, ml_algorithm: str, training_config: str, external_input: RegressionInputs) -> RegressionOutputs:
        try:
            if not self.model_dict["models"]:
                self.logger.error("Please load some models")
                raise RuntimeError("Models have not been loaded. Please initialize the models.")

            ml_algorithm_dict = self.model_dict["models"].get(ml_algorithm)
            ml_artifacts_dict = self.model_dict["artifacts"].get(ml_algorithm)
            ml_config_dict = self.model_dict["config"].get(ml_algorithm)

            if not ml_algorithm_dict or not ml_artifacts_dict or not ml_config_dict:
                self.logger.error(f"No ML Algorithm called {ml_algorithm}")
                raise RuntimeError(f"No ML Algorithm called {ml_algorithm}")

            model = ml_algorithm_dict.get(training_config)
            transformer = ml_artifacts_dict.get(training_config, {}).get("transformer")
            scaler = ml_artifacts_dict.get(training_config, {}).get("scaler")
            config = ml_config_dict.get(training_config)

            feature_extractor = FeatureExtractor(config)
            input_columns = feature_extractor.get_input_features_list()
            target_column = feature_extractor.get_numerical_outputs_list()[0]

            if target_column in input_columns:
                input_columns.remove(target_column)

            data_dict = external_input.__root__
            correlation_ids = list(data_dict.keys())
            data_values = list(data_dict.values())

            self.logger.info(f"Expected input columns: {input_columns}")
            df_input = pd.DataFrame(data_values, columns=input_columns)

            self.logger.info(f"Columns provided during inference: {df_input.columns}")
            preprocessed_data = transformer.transform(df_input)

            predictions = model.predict(preprocessed_data)

            if scaler:
                predictions = scaler.inverse_transform(predictions.reshape(-1, 1)).flatten()

            prediction_dict = {
                correlation_ids[i]: predictions[i]
                for i in range(len(correlation_ids))
            }

            return RegressionOutputs(__root__=prediction_dict)
        except Exception as e:
            traceback.print_exc()
            self.logger.error(f"Prediction failed: {e}")
            raise RuntimeError(f"Prediction failed: {e}")


def run_watcher(inference_engine: HedgeInferenceBase):
    global shared_engine
    start_fs_monitor(inference_engine)


if __name__ == "__main__":
    inference_obj = LightGBMRegressionInference()
    inference_obj.load_model(inference_obj.model_dir, inference_obj.artifacts_dict)

    run_watcher(inference_engine=inference_obj)

    reg_inputs = RegressionInputs(__root__={
        "correlation-id-01": [11, 12],
        "correlation-id-02": [14, 15]
    })

    reg_outputs = RegressionOutputs(__root__={})

    app = InferenceAPI(inference_obj, inputs=reg_inputs, outputs=reg_outputs)
    uvicorn.run(app, host=inference_obj.host, port=int(inference_obj.port))
