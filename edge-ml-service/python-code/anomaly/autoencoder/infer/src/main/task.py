"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from typing import List, Union, Dict
import os
import traceback
import uvicorn
import pandas as pd
import numpy as np

from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.ml.hedge_inference import HedgeInferenceBase, Inputs, Outputs
from common.src.util.config_extractor import FeatureExtractor
from common.src.ml.hedge_api import InferenceAPI
from common.src.ml import start_fs_monitor


class AutoEncoderInputs(Inputs):
    """
        Defines the input payload for Autoencoder
        """
    __root__: dict[str, List[Union[float, str]]]

class AutoEncoderOutputs(Outputs):
    """
    Defines the output payload for Autoencoder
    """
    __root__: Dict[str, float]

class AutoEncoderInference(HedgeInferenceBase):
    """
        This class handles the inference process for the autoencoder model used in anomaly detection.
        It includes model loading, data preprocessing, prediction, and post-processing.
    """
    model_dir = None
    logger = None
    model_dict = None
    port = None
    host = None

    def __init__(self):
        self.logger = LoggerUtil().logger

        env_file: str = os.path.join(os.getcwd(), "anomaly", "autoencoder", "env.yaml")
        self.env_util = EnvironmentUtil(env_file)

        super().get_env_vars()

        self.artifacts_dict = {
            "models": "model.gz",
            "artifacts": "artifacts.gz",
            "config": "assets/config.json"
        }

        super().clear_and_initialize_model_dict(self.artifacts_dict.keys())

    def read_model_config(self):
        pass

    def predict(self, ml_algorithm: str, training_config: str, external_input: Inputs) -> AutoEncoderOutputs:
        try:
            if not self.model_dict["models"]:
                self.logger.error("Please load some models")
                raise RuntimeError("Models have not been loaded. Please initialize the models.")

            ml_algorithm_dict = self.model_dict["models"].get(ml_algorithm, None)
            ml_artifacts_dict = self.model_dict["artifacts"].get(ml_algorithm, None)
            ml_config_dict = self.model_dict["config"].get(ml_algorithm, None)
            if not ml_algorithm_dict:
                self.logger.error(
                    f"No ML Algorithm called {ml_algorithm}, available algorithms: {self.model_dict['models']}")
                raise RuntimeError(f"No ML Algorithm called {ml_algorithm}")

            model = ml_algorithm_dict.get(training_config, None)
            if not model:
                self.logger.error(f"No model called {training_config}")
                raise RuntimeError(f"The model {training_config} does not exist")

            training_artifact = ml_artifacts_dict.get(training_config, None)
            if not training_artifact:
                self.logger.error(f"No training artifact called {training_artifact}")
                raise RuntimeError(f"The training artifact {training_artifact} does not exist")

            mse_median = training_artifact.get("mse_median", None)
            median_absolute_deviation = training_artifact.get("median_absolute_deviation", None)
            transformer = training_artifact.get("transformer", None)

            config = ml_config_dict.get(training_config, None)
            features_extractor = FeatureExtractor(config)

            features_dict = features_extractor.get_data_object()["featureNameToColumnIndex"]
            features_dict = {key: value for key, value in sorted(features_dict.items(), key=lambda item: item[1])}

            data_dict = external_input.__root__
            correlation_ids = list(data_dict.keys())
            data_values = list(data_dict.values())

            df_input = pd.DataFrame(data_values, columns=features_dict.keys())
            preprocessed_data = transformer.transform(df_input)

            predictions = model.predict(preprocessed_data)

            postprocessed_predictions = self.__postprocess_predictions(preprocessed_data, predictions, mse_median,
                                                                       median_absolute_deviation)

            prediction_dict = {
                correlation_ids[i]: postprocessed_predictions[i]
                for i in range(len(correlation_ids))
            }
            final_predictions = AutoEncoderOutputs(__root__=prediction_dict)

            return final_predictions
        except Exception as e:
            traceback.print_exc()
            self.logger.error(f"Prediction failed: {e}")

    def __postprocess_predictions(self, inputs, outputs, mse_median, median_absolute_deviation) -> List[float]:
        mse_list = np.mean(np.power(np.array(inputs) - np.array(outputs), 2), axis=1)
        z_scores = [
            self.__modified_z_score_scoring(mse, mse_median, median_absolute_deviation)
            for mse in mse_list
        ]
        return z_scores

    def __modified_z_score_scoring(self, points, mse_median, median_absolute_deviation) -> float:
        absolute_deviation = np.abs(points - mse_median)
        if median_absolute_deviation == 0:
            self.logger.warning("Median Absolute Deviation is zero. Returning zero as Z-score.")
            return 0
        return 0.6745 * absolute_deviation / median_absolute_deviation

def run_watcher(inference_engine: HedgeInferenceBase):
    global shared_engine
    start_fs_monitor(inference_engine)


if __name__ == "__main__":
    inference_engine = AutoEncoderInference()
    inference_engine.load_model(inference_engine.model_dir, inference_engine.artifacts_dict)

    run_watcher(inference_engine=inference_engine)

    auto_inputs = AutoEncoderInputs(__root__={
        "correlation-id-01": [1, 2, 3],
        "correlation-id-02": [3, 4, 4],
        "correlation-id-03": [3, 4, 5]
    })

    auto_outputs = AutoEncoderOutputs(__root__={})

    app = InferenceAPI(inference_engine, inputs=auto_inputs, outputs=auto_outputs)

    uvicorn.run(app, host=inference_engine.host, port=int(inference_engine.port))
