"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from __future__ import absolute_import, division, print_function

import math
import os
import shutil
import traceback

import joblib
import numpy as np
import pandas as pd
import tensorflow as tf
from sklearn.model_selection import train_test_split

from common.src.util.exceptions import HedgeTrainingException
from common.src.util.config_extractor import FeatureExtractor
from common.src.ml.hedge_training import HedgeTrainingBase
from common.src.util.data_transformer import DataFramePreprocessor
from common.src.util.env_util import EnvironmentUtil
from common.src.util.logger_util import LoggerUtil
from common.src.ml.hedge_status import Status



class Autoencoder(HedgeTrainingBase):
    def __init__(self):
        self.logger_util = LoggerUtil()
        self.logger = self.logger_util.logger

        # Pass this as environment variable (local, model_dir)
        self.local = False
        self.data_util = None

        self.model_dir = None
        env_file: str = os.path.join(os.getcwd(), "anomaly", "autoencoder", "env.yaml")
        # args to be removed
        self.env_util = EnvironmentUtil(env_file, args=['TRAINING_FILE_ID'])

        self.base_path = ""
        self.algo_name = ""
        self.artifact_dictionary = {}

        # self.encoder_model = self.encoders
        self.perform_rca = True
        self.num_epochs = 40
        self.batch_size = 128
        self.learning_rate = 0.01
        self.min_delta = 0.025
        self.patience = 5
        self.threshold = 2
        self.verbose = 1

        self.num_of_layers = 0
        self.num_of_neurons = 0
        self.num_epochs_run = 0
        self.loss_value = 0
        self.mse = 0
        self.mse_median = 0
        self.absolute_deviation = 0
        self.median_absolute_deviation = 0

        self.encoders = None
        self.scaler = None
        self.model = None

        self.input_size = 4


    def save_artifacts(self, artifact_dictionary, model):
        try:
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)

            self.logger.info(f"Attempting to save artifacts at: {export_path}")
            self.logger.info(f"artifact_dictionary: {artifact_dictionary}")

            # Save the artifact dictionary
            joblib.dump(artifact_dictionary, f"{export_path}/artifacts.gz", compress=True)
            self.logger.info(f"Artifact dictionary saved successfully at: {export_path}/artifacts.gz")

            # Save the model
            joblib.dump(model, f"{export_path}/model.gz", compress=True)
            self.logger.info(f"Model saved successfully at: {export_path}/model.gz")

            # Zip and upload model
            os.chdir(self.model_dir)
            model_zip_file = shutil.make_archive("hedge_export", "zip", root_dir=".", base_dir=f"./{self.algo_name}")
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
            features_extractor = FeatureExtractor(self.load_json(full_config_json_path))
            features_dict = features_extractor.get_data_object()["featureNameToColumnIndex"]
            self.logger.info(f"Features: {features_dict=}")

            if not features_dict or len(features_dict)==0:
                raise KeyError("No features found in the configuration file.")

            features_dict = {key: value for key, value in sorted(features_dict.items(), key=lambda my_dict: my_dict[1])}
            self.logger.info(f"Features Sorted: {features_dict=}")

            self.pipeline_status.update(Status.INPROGRESS, "Start of Training")
            # Read the training data
            df = pd.read_csv(full_csv_file_path, usecols=features_dict.keys())
            df = df.fillna(0.0)  # Fallback handling for missing values

            # Initialize the custom DataFramePreprocessor
            preprocessor = DataFramePreprocessor()

            # Fit the preprocessor on the training data and transform the train/validate sets
            df_train, df_validate = train_test_split(df, test_size=0.2, random_state=42)
            df_train_transformed = preprocessor.fit_transform(df_train)
            df_validate_transformed = preprocessor.transform(df_validate)

            # Model architecture definition
            self.input_size = df_train_transformed.shape[1]
            inner_layer_sizes = self.compute_inner_layer_sizes()
            activation_function = "elu"
            kernel_initializer = "glorot_uniform"

            # define Autoencoder using Tensorflow's Functional API and incorporate normalization within the model
            input_layer = tf.keras.Input(
                shape=(df_train_transformed.shape[1],), name="inputs"
            )
            # encoder block
            x = tf.keras.layers.Dense(
                units=df_train_transformed.shape[1],
                activation=activation_function,
                kernel_initializer=kernel_initializer,
            )(input_layer)
            # dynamically sized compression
            for i in inner_layer_sizes:
                x = tf.keras.layers.Dense(
                    units=i,
                    activation=activation_function,
                    kernel_initializer=kernel_initializer,
                )(x)
            # decoder block
            for i in reversed(inner_layer_sizes[:-1]):
                x = tf.keras.layers.Dense(
                    units=i,
                    activation=activation_function,
                    kernel_initializer=kernel_initializer,
                )(x)
            output_layer = tf.keras.layers.Dense(
                units=df_train_transformed.shape[1],
                activation=activation_function,
                kernel_initializer=kernel_initializer,
            )(x)

            # model graph
            model = tf.keras.models.Model(
                inputs=input_layer, outputs=output_layer, name="autoencoder"
            )

            # model configuration
            model.compile(optimizer="adam", loss="mse", metrics=["acc"])

            self.num_of_layers = len(model.layers)
            self.num_of_neurons = sum([layer.count_params() for layer in model.layers])

            # define early stopping for regularization
            early_stop = tf.keras.callbacks.EarlyStopping(
                monitor="loss",
                min_delta=self.min_delta,
                patience=self.patience,
                verbose=self.verbose,
                mode="min",
                restore_best_weights=True,
            )
            cb = [early_stop]

            # Train the model
            self.logger.info("Starting Training Process")
            history = model.fit(
                df_train_transformed,
                df_train_transformed,
                shuffle=False,
                epochs=self.num_epochs,
                batch_size=self.batch_size,
                callbacks=cb,
                validation_data=(df_validate_transformed, df_validate_transformed),
            )
            # Check if the training completed
            self.logger.info(f"Training completed, history: {history.history}")

            # Collect how many Epochs it ran for
            self.num_epochs_run = len(history.history["loss"])

            # pass the transformed test set through the autoencoder to get the reconstructed result
            reconstructions = model.predict(df_validate_transformed)

            # calculating the mean squared error reconstruction loss per row in the numpy array
            self.mse = np.mean(
                np.power(df_validate_transformed - reconstructions, 2), axis=1
            )
            self.mse_median = np.median(self.mse)
            self.absolute_deviation = np.abs(self.mse - self.mse_median)
            self.median_absolute_deviation = np.median(self.absolute_deviation)

            # Evaluate the model on validation data to get loss
            self.loss_value = model.evaluate(
                df_validate_transformed, df_validate_transformed
            )
            # Save the preprocessor, scaler, and other artifacts
            self.artifact_dictionary = {
                "transformer": preprocessor.get_preprocessor(),
                "scaler": preprocessor.get_standard_scaler(),
                "threshold": self.threshold,
                "mse_median": self.mse_median,
                "median_absolute_deviation": self.median_absolute_deviation,
            }
            self.model = model
        except ValueError as e:
            raise HedgeTrainingException("Training failed due to invalid data: {e}", error_code=2001) from e
        except KeyError as e:
            raise HedgeTrainingException("Training failed due to missing features", error_code=2003) from e
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
            config_file_path = f"{path_to_training_data}/config.json"

            shutil.copy2(config_file_path, asset_path)

            summary_text = (
                "Model Statistics:\n"
                f"# of Layers: {self.num_of_layers}\n"
                f"Total # of Neurons: {self.num_of_neurons}\n"
                f"Total # of Epochs ran before Stopping: {self.num_epochs_run}\n"
                "\n"
                "Training Statistics:\n"
                f"Loss: {self.loss_value}\n"
                f"MSE: {self.mse}\n"
                f"MSE Median: {self.mse_median}\n"
                f"Absolute Deviation: {self.absolute_deviation}\n"
                f"Median Absolute Deviation: {self.median_absolute_deviation}\n"
            )

            with open(f"{asset_path}/training_summary.txt", "w") as model_summary:
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

            self.batch_size = int(self.env_util.get_env_value("BATCH_SIZE", str(self.batch_size)))
            self.num_epochs = int(self.env_util.get_env_value("NUM_EPOCH", str(self.num_epochs)))
        except KeyError as e:
            raise HedgeTrainingException("Environment variable is missing", error_code=1001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in get_env_vars: {e}", error_code=1002) from e


    def execute_training_pipeline(self, max_retries=3):
        """
        Executes the training pipeline with a mechanism to retrain if mse_median or
        median_absolute_deviation are zero. Retrains up to max_retries times.
        """
        try:
            # Sets Model Environment Variables, Local Flag, & Config Toml
            self.logger.info("Getting Environment Variables")
            self.get_env_vars()


            retries = 0
            while retries <= max_retries:
                # Build & Train the autoencoder
                self.logger.info("Training Autoencoder...")
                self.train()

                if self.mse_median == 0 or self.median_absolute_deviation == 0:
                    retries += 1
                    self.logger.warning(
                        f"mse_median or median_absolute_deviation is zero. Retrying training ({retries}/{max_retries})..."
                    )

                    if retries > max_retries:
                        self.logger.error("Training failed: Reached max retries with zero mse_median or median_absolute_deviation.")
                        raise ZeroDivisionError("Training failed: Zero mse_median or median_absolute_deviation after 3 training attempts.")
                        break
                    else:
                        continue
                else:
                    break

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

    def compute_inner_layer_sizes(self):
        """Compute inner layer sizes for autoencoder"""
        try:
            if self.input_size <= 4:
                encoder_start_power = round(math.log2(4))
            else:
                encoder_start_power = round(math.log2(self.input_size))

            if encoder_start_power >= 9:
                bottleneck_power = encoder_start_power - 4
            elif encoder_start_power >= 5:
                bottleneck_power = encoder_start_power - 3
            else:
                bottleneck_power = 1

            return 2 ** np.arange(start=encoder_start_power, stop=bottleneck_power - 1, step=-1)
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in compute_inner_layer_sizes: {e}", error_code=5001) from e


if __name__ == "__main__":
    Autoencoder().execute_training_pipeline()
