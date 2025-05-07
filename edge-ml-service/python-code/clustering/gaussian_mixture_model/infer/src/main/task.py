"""
(c) Copyright 2020-2025 BMC Software, Inc.
Contributors: BMC Software, Inc. - BMC Helix Edge
"""

import os
import pandas as pd
import uvicorn
import numpy as np
import traceback
from typing import List, Union, Dict

from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.ml.hedge_inference import HedgeInferenceBase, Inputs, Outputs
from common.src.util.config_extractor import FeatureExtractor
from common.src.ml.hedge_api import InferenceAPI
from common.src.ml import start_fs_monitor


class GMMInputs(Inputs):
    """Defines the input payload for GaussianMixture clustering."""
    __root__: dict[str, List[Union[float, str]]]


class GMMOutputs(Outputs):
    """Defines the output payload for GaussianMixture clustering."""
    __root__: Dict[str, Dict[str, Union[str, float]]]


class GMMInference(HedgeInferenceBase):
    """
    This class handles the inference process for the Gaussian Mixture Model used in clustering.
    It includes model loading, data preprocessing, prediction, and post-processing.
    """
    model_dir = None
    logger = None
    model_dict = None
    port = None
    host = None

    def __init__(self):
        self.logger = LoggerUtil().logger

        env_file: str = os.path.join(os.getcwd(), "clustering", "gaussian_mixture_model", "env.yaml")
        self.env_util = EnvironmentUtil(env_file)

        super().get_env_vars()
        self.port = self.env_util.get_env_value('PORT', self.env_util.get_env_value('Service.port', '52000'))

        self.artifacts_dict = {
            "models": "model.gz",
            "artifacts": "artifacts.gz",
            "config": "assets/config.json"
        }

        super().clear_and_initialize_model_dict(self.artifacts_dict.keys())

    def read_model_config(self):
        pass

    def predict(self, ml_algorithm: str, training_config: str, external_input: GMMInputs) -> GMMOutputs:
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
            config = ml_config_dict.get(training_config)

            feature_extractor = FeatureExtractor(config)
            input_features = feature_extractor.get_input_features_list()

            data_dict = external_input.__root__
            correlation_ids = list(data_dict.keys())
            data_values = list(data_dict.values())

            df_input = pd.DataFrame(data_values, columns=input_features)
            preprocessed_data = transformer.transform(df_input)

            cluster_assignments = model.predict(preprocessed_data)
            probabilities = model.predict_proba(preprocessed_data)
            entropies = self.calculate_entropy(probabilities)

            # Construct the output dictionary
            prediction_dict = {
                correlation_ids[i]: {
                    "ClusterName": f"Cluster-{cluster_assignments[i]}",
                    "entropy": entropies[i]
                }
                for i in range(len(correlation_ids))
            }

            return GMMOutputs(__root__=prediction_dict)
        except Exception as e:
            traceback.print_exc()
            self.logger.error(f"Prediction failed: {e}")
            raise RuntimeError(f"Prediction failed: {e}")

    def calculate_entropy(self, probabilities: np.ndarray) -> List[float]:
        return [-np.sum(prob * np.log(prob + 1e-10)) for prob in probabilities]


def run_watcher(inference_engine: HedgeInferenceBase):
    global shared_engine
    start_fs_monitor(inference_engine)


if __name__ == "__main__":
    inference_engine = GMMInference()
    inference_engine.load_model(inference_engine.model_dir, inference_engine.artifacts_dict)

    run_watcher(inference_engine=inference_engine)

    gmm_inputs = GMMInputs(__root__={
        "correlation-id-01": [1, 2, 3],
        "correlation-id-02": [3, 4, 4],
        "correlation-id-03": [3, 4, 5]
    })

    gmm_outputs = GMMOutputs(__root__={})

    app = InferenceAPI(inference_engine, inputs=gmm_inputs, outputs=gmm_outputs)
    uvicorn.run(app, host=inference_engine.host, port=int(inference_engine.port))
