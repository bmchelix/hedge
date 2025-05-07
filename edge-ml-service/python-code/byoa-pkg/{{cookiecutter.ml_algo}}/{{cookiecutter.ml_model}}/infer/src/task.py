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
from common.src.util.infer_exception import HedgeInferenceException

{% if cookiecutter.algo_type == "Anomaly" %}
# Template code for Anomaly
class {{ cookiecutter.infer_class_name }}Inputs(Inputs):
    __root__: dict[str, List[Union[float, str]]]

class {{ cookiecutter.infer_class_name }}Outputs(Outputs):
    __root__: Dict[str, float]

{% elif cookiecutter.algo_type == "Regression" %}
# Template code for Regression
class {{ cookiecutter.infer_class_name }}Inputs(Inputs):
    __root__: dict[str, List[Union[float, str]]]

class {{ cookiecutter.infer_class_name }}Outputs(Outputs):
    __root__: Dict[str, float]

{% elif cookiecutter.algo_type == "Classification" %}
# Template code for Regression
class {{ cookiecutter.infer_class_name }}Inputs(Inputs):
    __root__: dict[str, List[Union[float, str]]]

class {{ cookiecutter.infer_class_name }}Outputs(Outputs):
    __root__: Dict[str, Dict[str, Union[str, float]]]
    def dict(self, **kwargs):
        """Override serialization to ensure confidence stays a float."""
        original_dict = super().dict(**kwargs)
        for key, value in original_dict['__root__'].items():
            value['confidence'] = float(value['confidence'])
        return original_dict

{% else %}
# Template code for everything else
class {{ cookiecutter.infer_class_name }}Inputs(Inputs):
    __root__: Dict[str, Any]

class {{ cookiecutter.infer_class_name }}Outputs(Outputs):
    __root__: Dict[str, Any]
{% endif %}



class {{cookiecutter.infer_class_name}}(HedgeInferenceBase):
    
    def __init__(self):
        self.logger = LoggerUtil().logger
        self.env_util = EnvironmentUtil(os.getcwd() + '/{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/env.yaml')

        super().get_env_vars()
        self.port = self.env_util.get_env_value('PORT', self.env_util.get_env_value('Service.port', '55000'))
        
        self.artifacts_dict = {"models": "model.gz",
                               "artifacts": "artifacts.gz",
                               #"params": "model_params_dict.gz",
                               "config": "assets/config.json"}

        super().clear_and_initialize_model_dict(self.artifacts_dict.keys())

    def read_model_config(self):
        pass

    
    def predict(self, ml_algorithm: str, training_config: str, external_input: {{cookiecutter.infer_class_name}}Inputs) -> {{cookiecutter.infer_class_name}}Outputs:
        try:
            if not self.model_dict["models"]:
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
                self.logger.error(f"No training artifact found for {training_config}")
                raise RuntimeError(f"The training artifact for {training_config} does not exist")

            ##TODO - Pull out the additional artifacts for use
            # i.e. transformer = training_artifact.get("transformer", None)

            config = ml_config_dict.get(training_config, None)
            if not config:
                self.logger.error(f"No config called found for {training_config}")
                raise RuntimeError(f"The config for {training_config} does not exist")

            features_extractor = FeatureExtractor(config)

            features_dict = features_extractor.get_data_object()["featureNameToColumnIndex"]
            features_dict = {key: value for key, value in sorted(features_dict.items(), key=lambda item: item[1])}

            data_dict = external_input.__root__
            correlation_ids = list(data_dict.keys())
            self.logger.info(f"Correlation IDs: {correlation_ids}")
            data_values = list(data_dict.values())
            
            if len(correlation_ids) == 0:
                raise HedgeInferenceException(status_code=400, detail="Correlation IDs list is empty")

            #ml_params_dict = self.model_dict["params"].get(ml_algorithm, {})
            #params = ml_params_dict.get(training_config, None)
            
            #if predictor is None or params is None:
            #    raise HedgeInferenceException(status_code=404, detail="No model found")

            #target_columns = params["target_columns"]
            #group_by_cols = params["group_by_cols"]
            
            ##TODO - START of Inference related logic 

            results = {}
            ##END of Inference related logic

            
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
    
    model_inputs = {{cookiecutter.infer_class_name}}Inputs(__root__={
        ## TODO - Define your sample data object here
        # i.e.
        "correlation-id-01": [1, 2, 3]
    })
    
    try:
        inference_obj = {{cookiecutter.infer_class_name}}()
        inference_obj.load_model(inference_obj.model_dir, inference_obj.artifacts_dict)
    except Exception as e:
        raise Exception(f"Unexpected error while loading models: {e}")

    model_outputs = {{cookiecutter.infer_class_name}}Outputs(__root__={})

    try:  
        app = InferenceAPI(inference_obj, inputs=model_inputs, outputs=model_outputs)
        uvicorn.run(app, host=inference_obj.host, port=int(inference_obj.port))
    except Exception as e:
        raise Exception(f"Unexpected error while starting the Inference container: {e}")