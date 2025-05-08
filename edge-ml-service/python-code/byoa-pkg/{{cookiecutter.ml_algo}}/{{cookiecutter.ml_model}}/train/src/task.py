import traceback
import joblib
import json
import shutil
import os
import pandas as pd
import numpy as np 

from common.src.util.exceptions import HedgeTrainingException
from common.src.ml.hedge_training import HedgeTrainingBase
from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.util.config_extractor import FeatureExtractor
from common.src.ml.hedge_status import Status

class {{cookiecutter.training_class_name}}(HedgeTrainingBase):
    logger = None
    freq = None
    train_data = None
    test_data = None
    target_columns = None
    input_columns = None
    output_columns = None
    feature_extractor = None
    date_field = None
    group_by_cols = None
    
    def __init__(self):
        self.logger = LoggerUtil().logger
        self.local = False
        self.data_util = None
        self.model_dir = None 

        env_file: str = os.path.join(os.getcwd(), "{{cookiecutter.ml_algo}}", "{{cookiecutter.ml_model}}", "env.yaml")
        self.env_util = EnvironmentUtil(
            env_file, args=["TRAINING_FILE_ID"]
        )

        self.metrics_dict = {}
        self.artifacts_dict = {}
        self.model = None
        self.feature_extractor = None     

    def save_artifacts(self):
        try:
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)

            self.logger.info(f"Attempting to save artifacts at: {export_path}")
            self.logger.info(f"artifacts_dict: {self.artifacts_dict}")

            # Save the artifact dictionary
            joblib.dump(self.artifacts_dict, f'{export_path}/artifacts.gz', compress=True)
            self.logger.info(f"Artifact dictionary saved successfully at: {export_path}/artifacts.gz")

            # Save the model
            joblib.dump(self.model, f'{export_path}/model.gz', compress=True)
            self.logger.info(f"Model saved successfully at: {export_path}/model.gz")

            # Zip and upload model
            os.chdir(self.model_dir)
            model_zip_file = shutil.make_archive('hedge_export', 'zip', root_dir=".", base_dir=f'./{self.algo_name}')
            self.data_util.upload_data(model_zip_file)

        except IOError as e:
            raise HedgeTrainingException("Failed to save artifacts", error_code=4001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in save_artifacts: {e}", error_code=4002) from e
        
    def train(self):
        try:
            # Read environment variables
            self.get_env_vars()
            
            data_source_info = self.data_util.read_data()
            self.algo_name = data_source_info.algo_name
            full_config_json_path = data_source_info.config_file_path
            full_csv_file_path = data_source_info.csv_file_path

            with open(full_config_json_path, 'r') as f:
                config_dict = json.load(f)

            self.feature_extractor = FeatureExtractor(data=config_dict)

            self.input_columns = self.feature_extractor.get_input_features_list()
            self.logger.info(f"Input columns: {self.input_columns}")
            self.output_columns = self.feature_extractor.get_output_features_list()
            self.logger.info(f"Output columns: {self.output_columns}")
            
            if len(self.input_columns) == 0 or len(self.output_columns) == 0:
                raise ValueError("No input features or target column found in the config.json")
            
            self.pipeline_status.update(Status.INPROGRESS, "Start of Training")
            
            ##TODO - Update the training logic below 
            df = pd.read_csv(full_csv_file_path)
            
            
            self.metrics_dict = {
                ##TODO - Update the metrics here 
                # 'rmse': rmse,
                # 'relative_rmse': relative_rmse
            }
            
            self.artifacts_dict = {
                ##TODO - Update the additional artifacts - preprocessor or scaler
                # 'transformer': preprocessor,
                # 'scaler': target_scaler,                
            }
            
            # TODO - Assign the model object here
            self.model = None
            ## END of Training logic
            
            self.logger.info("Model training completed successfully")
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in train: {e}", error_code=2002) from e     

    def create_and_save_summary(self):
        
        try:
            ##TODO - User self.metrics_dict to update any additional metrics
            # self.metrics_dict['KEY'] = VALUE
            
            
            ## END of metrics logic
            self.logger.info(f"Evaluation Metric Summary: {self.metrics_dict}")
            path_to_training_data = f"{self.data_util.base_path}/data"
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)
            asset_path = os.path.join(f"{export_path}", "assets")
            os.makedirs(asset_path, exist_ok=True)
            config_file_path = f'{path_to_training_data}/config.json'
            shutil.copy2(config_file_path, asset_path)
            
            summary_text = json.dumps(self.metrics_dict, indent=4)
            with open(f'{asset_path}/training_summary.txt', "w") as model_summary:
                model_summary.write(summary_text)
            self.logger.info(f"Training summary saved to {asset_path}/training_summary.txt")
        except RuntimeError as e:
            raise HedgeTrainingException("Failed to generate summary", error_code=3001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in create_summary: {e}", error_code=3002) from e

    def get_env_vars(self):
        try:
            """Reads and returns environment variables"""
            super().get_env_vars()
            
            ##TODO - Populate any additional variables read from either env vars or from env.yaml file
            # self.num_epochs = int(self.env_util.get_env_value(
            #     "NUM_EPOCH", 10
            # ))  
            
        except KeyError as e:
            raise HedgeTrainingException("Environment variable is missing", error_code=1001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in get_env_vars: {e}", error_code=1002) from e        
    
    
    def execute_training_pipeline(self):
        try:
            # Sets Model Environment Variables, Local Flag, & Config Toml
            self.logger.info("Getting Environment Variables")
            self.get_env_vars()

            self.logger.info("Training {{cookiecutter.ml_algo}} - {{cookiecutter.ml_model}}...")
            self.train()

            # Generate Training Summary
            self.logger.info("Generating Training Summary...")
            self.create_and_save_summary()

            # Export & Zip Model
            self.logger.info("Exporting Model...")
            self.save_artifacts()
            self.pipeline_status.update(Status.SUCCESS, "End of Training")
        except HedgeTrainingException as te:
            self.logger.error(f"Training pipeline failed with error: {te}")
            self.logger.error(traceback.format_exc())
            self.pipeline_status.update(Status.FAILURE, f"Training pipeline failed with error: {te}")
        except Exception as e:
            self.logger.error(f"An unexpected error occurred: {str(e)}")
            self.logger.error(traceback.format_exc())
            self.pipeline_status.update(Status.FAILURE, f"An unexpected error occurred: {str(e)}")
        finally:
            self.data_util.update_status(self.pipeline_status.is_success, self.pipeline_status.message)        


if __name__ == "__main__":
    {{cookiecutter.training_class_name}}().execute_training_pipeline()
