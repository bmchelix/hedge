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


class RandomForestInputs(Inputs):
    """
    Defines the input payload for Random Forest Classification
    """
    __root__: dict[str, List[Union[float, str]]]


class RandomForestOutputs(Outputs):
    __root__: Dict[str, Dict[str, Union[str, float]]]
    def dict(self, **kwargs):
        """Override serialization to ensure confidence stays a float."""
        original_dict = super().dict(**kwargs)
        for key, value in original_dict['__root__'].items():
            value['confidence'] = float(value['confidence'])
        return original_dict


class RandomForestInference(HedgeInferenceBase):
    """
    This class handles the inference process for the Random Forest model used in classification tasks.
    It includes model loading, data preprocessing, prediction, and output formatting.
    """
    model_dir = None
    logger = None
    model_dict = None
    port = None
    host = None
    artifacts_dict = None

    def __init__(self):
        self.logger = LoggerUtil().logger

        env_file: str = os.path.join(os.getcwd(), "classification", "random_forest", "env.yaml")
        self.env_util = EnvironmentUtil(env_file)

        super().get_env_vars()
        self.port = self.env_util.get_env_value('PORT', self.env_util.get_env_value('Service.port', '49096'))

        self.artifacts_dict = {
            "models": "model.gz",
            "artifacts": "artifacts.gz",
            "config": "assets/config.json"
        }

        super().clear_and_initialize_model_dict(self.artifacts_dict.keys())

    def read_model_config(self):
        pass

    def predict(self, ml_algorithm: str, training_config: str, external_input: RandomForestInputs) -> RandomForestOutputs:
        try:
            if not self.model_dict["models"]:
                self.logger.error("Models have not been loaded. Please initialize the models.")
                raise RuntimeError("Models have not been loaded. Please initialize the models.")

            ml_algorithm_dict = self.model_dict["models"].get(ml_algorithm, None)
            ml_artifacts_dict = self.model_dict["artifacts"].get(ml_algorithm, None)
            ml_config_dict = self.model_dict["config"].get(ml_algorithm, None)

            if not ml_algorithm_dict or not ml_artifacts_dict or not ml_config_dict:
                self.logger.error(f"No ML Algorithm called {ml_algorithm}")
                raise RuntimeError(f"No ML Algorithm called {ml_algorithm}")

            model = ml_algorithm_dict.get(training_config, None)
            input_transformer = ml_artifacts_dict.get(training_config, None)['input_transformer']
            label_encoder = ml_artifacts_dict.get(training_config, None)['label_encoder']
            config = ml_config_dict.get(training_config, None)

            feature_extractor = FeatureExtractor(config)
            input_features = feature_extractor.get_input_features_list()

            data_dict = external_input.__root__
            correlation_ids = list(data_dict.keys())
            data_values = list(data_dict.values())

            df_input = pd.DataFrame(data_values, columns=input_features)
            preprocessed_data = input_transformer.transform(df_input)

            predictions = model.predict(preprocessed_data)
            prediction_probs = model.predict_proba(preprocessed_data)

            cat_predictions = label_encoder.inverse_transform(predictions)

            prediction_dict = {}
            for i in range(len(correlation_ids)):
                confidence_value = float(max(prediction_probs[i]))
                prediction_dict[correlation_ids[i]] = {
                    "class": str(cat_predictions[i]),
                    "confidence": confidence_value
                }

            final_predictions = RandomForestOutputs(__root__=prediction_dict)
            return final_predictions
        except Exception as e:
            traceback.print_exc()
            self.logger.error(f"Prediction failed: {e}")
            raise RuntimeError(f"Prediction failed: {e}")

def run_watcher(inference_engine: HedgeInferenceBase):
    global shared_engine
    start_fs_monitor(inference_engine)


if __name__ == "__main__":
    inference_obj = RandomForestInference()
    inference_obj.load_model(inference_obj.model_dir, inference_obj.artifacts_dict)

    run_watcher(inference_engine=inference_obj)

    rf_inputs = RandomForestInputs(__root__={
        "correlation-id-01": [14.0, 73, 9.5, 82.0, "partly cloudy", 1010.82, 2, "Winter", 3.5, "inland"],
        "correlation-id-02": [39.0, 96, 8.5, 71.0, "partly cloudy", 1011.43, 7, "Spring", 10.0, "inland"],
        "correlation-id-03": [30.0, 64, 7.0, 16.0, "clear", 1018.72, 5, "Spring", 5.5, "mountain"],
        "correlation-id-04": [38.0, 83, 1.5, 82.0, "clear", 1026.25, 7, "Spring", 1.0, "coastal"],
        "correlation-id-05": [27.0, 74, 17.0, 66.0, "overcast", 990.67, 1, "Winter", 2.5, "mountain"]
    })

    rf_outputs = RandomForestOutputs(__root__={})

    app = InferenceAPI(inference_obj, inputs=rf_inputs, outputs=rf_outputs)

    uvicorn.run(app, host=inference_obj.host, port=int(inference_obj.port))
