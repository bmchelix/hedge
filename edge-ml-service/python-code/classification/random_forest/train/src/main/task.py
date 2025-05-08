"""
(c) Copyright 2020-2024 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from __future__ import absolute_import, division, print_function

import traceback
import os
import shutil
import joblib

import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import classification_report, accuracy_score
from sklearn.preprocessing import LabelEncoder


from common.src.ml.hedge_training import HedgeTrainingBase
from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.util.data_transformer import DataFramePreprocessor
from common.src.util.config_extractor import FeatureExtractor
from common.src.util.exceptions import HedgeTrainingException
from common.src.ml.hedge_status import Status


class Classification(HedgeTrainingBase):
    def __init__(self):
        self.logger_util = LoggerUtil()
        self.logger = self.logger_util.logger

        # Pass this as environment variable (local, job_dir)
        self.local = False
        self.data_util = None

        env_file: str = os.path.join(os.getcwd(), "classification", "random_forest", "env.yaml")
        self.env_util = EnvironmentUtil(env_file, args=['TRAINING_FILE_ID'])

        self.base_path = ''
        self.algo_name = ''
        self.artifact_dictionary = {}

        # Model Parameters
        self.num_trees = 100
        self.max_depth = None
        self.min_samples_split = 2
        self.min_samples_leaf = 1
        self.verbose = 1

        self.accuracy = None
        self.classification_report = None

        self.model = None


    def save_artifacts(self, artifact_dictionary, model):
        try:
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)

            self.logger.info(f"Attempting to save artifacts at: {export_path}")
            self.logger.info(f"artifact_dictionary: {artifact_dictionary}")

            # Save the dictionary using joblib with .gz extension
            joblib.dump(artifact_dictionary, f'{export_path}/artifacts.gz', compress=True)
            self.logger.info(f'Artifact dictionary file created at: {export_path}/artifacts.gz')

            # Serialize the RandomForest model using joblib with .gz extension
            joblib.dump(model, f'{export_path}/model.gz', compress=True)
            self.logger.info(f'Model file created at: {export_path}/model.gz')

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
            self.logger.info("Getting Model Variables & Configurations")

            # Read the training data
            data_source_info = self.data_util.read_data()
            self.algo_name = data_source_info.algo_name
            full_config_json_path = data_source_info.config_file_path
            full_csv_file_path = data_source_info.csv_file_path

            # Initialize FeatureExtractor with the configuration data
            feature_extractor = FeatureExtractor(self.load_json(full_config_json_path))
            input_features = feature_extractor.get_input_features_list()
            self.logger.info(f"Input features extracted: {len(input_features)}")
            output_features = feature_extractor.get_output_features_list()
            self.logger.info(f"Output features extracted: {len(output_features)}")

            if len(output_features) != 1:
                raise ValueError("There should be exactly one output feature.")

            output_feature = output_features[0]
            
            self.pipeline_status.update(Status.INPROGRESS, "Start of Training")
            # Read the training data
            df = pd.read_csv(full_csv_file_path)
            
            # Check if the output feature data is empty
            if df[output_feature].isna().all():
                raise ValueError("Output feature data is empty")
            
            # Check if the output feature data has non-string or object data
            if pd.api.types.is_numeric_dtype(df[output_feature].dtype):
                raise ValueError("Output feature data cannot have non-string or non-object data")
            
            df = df.fillna(0.0)  # Fallback handling for missing values
            
            
            # Separate the input features and the target (output) feature
            X = df[input_features]  # Only input features
            self.logger.info(f"Shape of original X: {X.shape}")
            y = df[output_feature]  # Output feature (target column)

            # Initialize the preprocessors for input data
            input_preprocessor = DataFramePreprocessor()

            # Split the data into training and validation sets
            X_train, X_validate, y_train, y_validate = train_test_split(X, y, test_size=0.2, random_state=33)

            # Fit and transform the input training data
            self.logger.info(f"Shape of original X_train: {X_train.shape}")
            X_train_transformed = input_preprocessor.fit_transform(X_train)
            self.logger.info(f"Shape of transformed X_train: {X_train_transformed.shape}")

            # Continue with the rest of the training pipeline...
            X_validate_transformed = input_preprocessor.transform(X_validate)

            # Encode the target feature using LabelEncoder (no OneHotEncoder for target)
            label_encoder = LabelEncoder()
            y_train_transformed = label_encoder.fit_transform(y_train)
            y_validate_transformed = label_encoder.transform(y_validate)

            # Train RandomForest model
            model = RandomForestClassifier(
                n_estimators=self.num_trees,
                max_depth=self.max_depth,
                min_samples_split=self.min_samples_split,
                min_samples_leaf=self.min_samples_leaf,
                verbose=self.verbose
            )
            model.fit(X_train_transformed, y_train_transformed)  # Use encoded target labels

            # Predict and evaluate
            y_pred = model.predict(X_validate_transformed)
            self.accuracy = accuracy_score(y_validate_transformed, y_pred)
            self.classification_report = classification_report(y_validate_transformed, y_pred)

            # Save artifacts
            self.artifact_dictionary = {
                'input_transformer': input_preprocessor.get_preprocessor(),
                'label_encoder': label_encoder  # Save the label encoder to inverse-transform predictions later
            }
            self.model = model
        except ValueError as e:
            raise HedgeTrainingException(f"Training failed due to invalid data: {str(e)} ", error_code=2001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in train: {e}", error_code=2002) from e

    def create_and_save_summary(self):

        try:
            path_to_training_data = f"{self.data_util.base_path}/data"
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            self.logger.info(f"export path for local model file: {export_path}")
            os.makedirs(export_path, exist_ok=True)

            # create the target path if that doesn't exist
            asset_path = os.path.join(f"{export_path}", "assets")
            os.makedirs(asset_path, exist_ok=True)
            config_file_path = f'{path_to_training_data}/config.json'

            shutil.copy2(config_file_path, asset_path)

            summary_text = (
                "Training Statistics:\n"
                f"Accuracy: {self.accuracy}\n"
                f"Classification Report:\n{self.classification_report}\n"
            )

            with open(f'{asset_path}/training_summary.txt', "w") as model_summary:
                model_summary.write(summary_text)
            self.logger.info(f"Training summary saved to {asset_path}/training_summary.txt")
        except RuntimeError as e:
            raise HedgeTrainingException("Failed to generate summary", error_code=3001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in create_summary: {e}", error_code=3002) from e

    def __repr__(self):
        return f"num_trees: {self.num_trees}, max_depth: {self.max_depth}, " + \
            f"min_samples_leaf: {self.min_samples_leaf}, min_samples_split: {self.min_samples_split}"

    def get_env_vars(self):
        """Reads and returns environment variables"""
        try:
            super().get_env_vars()

            self.num_trees = self.env_util.get_env_value("NUM_TREES", self.num_trees)
            self.max_depth = self.env_util.get_env_value("MAX_DEPTH", self.max_depth)
            self.min_samples_split = self.env_util.get_env_value("MIN_SAMPLES_SPLIT", self.min_samples_split)
            self.min_samples_leaf = self.env_util.get_env_value("MIN_SAMPLES_LEAF", self.min_samples_leaf)
        except KeyError as e:
            raise HedgeTrainingException("Environment variable is missing", error_code=1001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in get_env_vars: {e}", error_code=1002) from e


    def execute_training_pipeline(self):
        try:
            # Sets Model Environment Variables, Local Flag, & Config Toml
            self.logger.info("Getting Environment Variables")
            self.get_env_vars()

            # Build & Train the Classifier
            self.logger.info("Training Classifier...")
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
    Classification().execute_training_pipeline()
