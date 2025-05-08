"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from __future__ import absolute_import, division, print_function

import traceback
import joblib
import json
import shutil
import os
import numpy as np
import pandas as pd
import lightgbm as lgb
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import mean_squared_error

from common.src.util.exceptions import HedgeTrainingException
from common.src.ml.hedge_training import HedgeTrainingBase
from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.util.data_transformer import DataFramePreprocessor
from common.src.util.config_extractor import FeatureExtractor
from common.src.ml.hedge_status import Status


class LightGBMRegression(HedgeTrainingBase):
    def __init__(self):
        self.logger_util = LoggerUtil()
        self.logger = self.logger_util.logger

        # Pass this as environment variable (local, model_dir)
        self.local = False
        self.data_util = None

        env_file: str = os.path.join(os.getcwd(), "regression", "lightgbm", "env.yaml")
        self.env_util = EnvironmentUtil(env_file, args=['TRAINING_FILE_ID'])

        self.base_path = ""
        self.algo_name = ""
        self.artifact_dictionary = {}

        self.num_boost_round = 100
        self.early_stopping_rounds = 10
        self.learning_rate = 0.01

        self.model = None
        self.feature_extractor = None

    def save_artifacts(self, artifact_dictionary, model):
        try:
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)

            self.logger.info(f"Attempting to save artifacts at: {export_path}")
            self.logger.info(f"artifact_dictionary: {artifact_dictionary}")

            # Save the artifact dictionary
            joblib.dump(artifact_dictionary, f'{export_path}/artifacts.gz', compress=True)
            self.logger.info(f"Artifact dictionary saved successfully at: {export_path}/artifacts.gz")

            # Save the model
            joblib.dump(model, f'{export_path}/model.gz', compress=True)
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
            if self.data_util is None:
                raise ValueError("data_util is None")

            self.logger.info("Getting Model Variables & Configurations")

            # Read the training data
            data_source_info = self.data_util.read_data()
            self.algo_name = data_source_info.algo_name
            full_config_json_path = data_source_info.config_file_path
            full_csv_file_path = data_source_info.csv_file_path

            # Extract features from the config file
            self.feature_extractor = FeatureExtractor(self.load_json(full_config_json_path))
            input_columns = self.feature_extractor.get_input_features_list()
            target_column = self.feature_extractor.get_numerical_outputs_list()[0]

            if len(input_columns) == 0 or len(target_column) == 0:
                raise ValueError("No input features or target column found in the config.json")

            if target_column in input_columns:
                input_columns.remove(target_column)

            # Read the training data
            self.logger.info(f"Input Columns: {input_columns}")
            self.logger.info(f"Target Column: {target_column}")

            self.pipeline_status.update(Status.INPROGRESS, "Start of Training")
            
            # Initialize the custom DataFramePreprocessor
            preprocessor = DataFramePreprocessor()

            # Read the training data
            df = pd.read_csv(full_csv_file_path)
            df = df.fillna(0.0)

            df_train, df_validate = train_test_split(df, test_size=0.2, random_state=42)

            df_train_transformed = preprocessor.fit_transform(df_train[input_columns])
            df_validate_transformed = preprocessor.transform(df_validate[input_columns])

            target_scaler = StandardScaler()

            self.logger.info(f"Target column before scaling: {df_train[target_column].mean()}")

            df_train[target_column] = target_scaler.fit_transform(df_train[[target_column]])
            df_validate[target_column] = target_scaler.transform(df_validate[[target_column]])

            self.logger.info(f"Target column after scaling: {df_train[target_column].mean()}")

            train_data = lgb.Dataset(df_train_transformed, label=df_train[target_column])
            validate_data = lgb.Dataset(df_validate_transformed,
                                        label=df_validate[target_column], reference=train_data)

            params = {
                'objective': 'regression',
                'metric': 'rmse',
                'learning_rate': self.learning_rate,
                'num_leaves': 31,
                'verbose': 1,
                'num_threads': -1
            }

            self.num_boost_round = min(self.num_boost_round, 50)
            self.early_stopping_rounds = min(self.early_stopping_rounds, 5)

            self.logger.info("Starting LightGBM Training Process")
            self.model = lgb.train(
                params,
                train_data,
                num_boost_round=self.num_boost_round,
                valid_sets=[validate_data],
                callbacks=[lgb.early_stopping(stopping_rounds=self.early_stopping_rounds)]
            )

            predictions = self.model.predict(df_validate_transformed)

            predictions_original_scale = target_scaler.inverse_transform(predictions.reshape(-1, 1)).flatten()
            df_validate[target_column] = target_scaler.inverse_transform(df_validate[[target_column]])

            self.logger.info(f"Target column after inverse scaling: {predictions_original_scale.mean()}")
            self.logger.info(f"Validation Target Column (original scale): {df_validate[target_column].head()}")

            rmse = np.sqrt(mean_squared_error(df_validate[target_column], predictions_original_scale))
            self.logger.info(f"Validation RMSE (original scale): {rmse}")

            mean_actual = df_validate[target_column].mean()
            self.logger.info(f"Mean Actual: {mean_actual}")

            if mean_actual <= 0 or np.isclose(mean_actual, 0):
                self.logger.warning("Mean of actual values is zero or negative, Relative RMSE is undefined.")
                relative_rmse = np.inf
            else:
                relative_rmse = (rmse / mean_actual) * 100

            self.logger.info(f"Relative RMSE: {relative_rmse}%")

            self.artifact_dictionary = {
                'transformer': preprocessor,
                'scaler': target_scaler,
                'rmse': rmse,
                'relative_rmse': relative_rmse
            }
        except ValueError as e:
            raise HedgeTrainingException("Training failed due to invalid data: {e}", error_code=2001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in train: {e}", error_code=2002) from e

    def create_and_save_summary(self):
        try:
            path_to_training_data = f"{self.data_util.base_path}/data"
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            self.logger.info(f"Export path for local model file: {export_path}")
            os.makedirs(export_path, exist_ok=True)

            # create the target path if that doesn't exist
            asset_path = os.path.join(f"{export_path}", "assets")
            os.makedirs(asset_path, exist_ok=True)
            config_file_path = f'{path_to_training_data}/config.json'

            shutil.copy2(config_file_path, asset_path)

            summary_text = (
                "Model Statistics:\n"
                f"Learning Rate: {self.learning_rate}\n"
                f"Boosting Rounds: {self.num_boost_round}\n"
                "\n"
                "Training Statistics:\n"
                f"Validation RMSE: {self.artifact_dictionary['rmse']}\n"
                f"Relative RMSE: {self.artifact_dictionary['relative_rmse']}%\n"
            )

            with open(f'{asset_path}/training_summary.txt', "w") as model_summary:
                model_summary.write(summary_text)
            self.logger.info(f"Training summary saved to {asset_path}/training_summary.txt")

        except RuntimeError as e:
            raise HedgeTrainingException("Failed to generate summary", error_code=3001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in create_summary: {e}", error_code=3002) from e

    def get_env_vars(self):
        """Reads and returns environment variables"""
        try:
            super().get_env_vars()
            self.learning_rate = self.env_util.get_env_value("LEARNING_RATE", self.learning_rate)
            self.num_boost_round = self.env_util.get_env_value("NUM_BOOST_ROUND", self.num_boost_round)
        except KeyError as e:
            raise HedgeTrainingException("Environment variable is missing", error_code=1001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in get_env_vars: {e}", error_code=1002) from e

    def execute_training_pipeline(self):
        try:
            # Sets Model Environment Variables, Local Flag, & Config Toml
            self.logger.info("Getting Environment Variables")
            self.get_env_vars()

            self.logger.info("Training LightGBM Regression...")
            self.train()

            # Generate Training Summary
            self.logger.info("Generating Training Summary...")
            self.create_and_save_summary()

            # Export & Zip Model
            self.logger.info("Exporting Model...")
            self.save_artifacts(self.artifact_dictionary, self.model)
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
    LightGBMRegression().execute_training_pipeline()
