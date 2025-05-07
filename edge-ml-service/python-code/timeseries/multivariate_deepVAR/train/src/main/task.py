import os
import json
import time
import shutil
import joblib
import optuna
import traceback
import pandas as pd
from gluonts.dataset.common import ListDataset
from gluonts.mx.model.deepvar import DeepVAREstimator
from gluonts.evaluation import MultivariateEvaluator, backtest_metrics
from gluonts.mx.trainer import Trainer
from common.src.ml.hedge_training import HedgeTrainingBase
from common.src.util.config_extractor import FeatureExtractor
from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.ml.hedge_status import Status
from timeseries.multivariate_deepVAR.train.src.main.tuning import DeepVARTuningObjective
from gluonts.dataset.field_names import FieldName
from common.src.util.exceptions import HedgeTrainingException

class TimeSeriesForecasting(HedgeTrainingBase):
    logger = None
    output_prediction_count = None
    input_context_count = None
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

        env_file: str = os.path.join(os.getcwd(), "timeseries", "multivariate_deepVAR", "env.yaml")
        self.env_util = EnvironmentUtil(
            env_file, args=["TRAINING_FILE_ID"]
        )

        self.group_by_cols = None
        self.output_prediction_count = None
        self.input_context_count = None
        self.base_path = ''
        self.algo_name = ''

    def get_env_vars(self):
        try:
            """Reads and returns environment variables"""
            super().get_env_vars()

            self.num_epochs = int(self.env_util.get_env_value(
                "NUM_EPOCH", 10
            ))
        except KeyError as e:
            raise HedgeTrainingException("Environment variable is missing", error_code=1001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in get_env_vars: {e}", error_code=1002) from e


    def _remove_inconsistent_records(self, df, datetime_column):
        """
        Retains the largest contiguous block of consistent records based on the most common time difference.
        Removes other records.

        Args:
            df (pd.DataFrame): Input DataFrame.
            datetime_column (str): Name of the datetime column.

        Returns:
            tuple: (Cleaned DataFrame, Bad Entries DataFrame)
        """

        # Ensure datetime_column is in datetime format
        df[datetime_column] = pd.to_datetime(df[datetime_column], errors='coerce')

        # Drop rows where datetime conversion failed
        df = df.dropna(subset=[datetime_column])

        # Sort by datetime
        df = df.sort_values(by=datetime_column).reset_index(drop=True)

        # Calculate time differences between consecutive rows
        df['time_diff'] = df[datetime_column].diff()

        # Determine the most common time difference
        most_common_diff = df['time_diff'].mode().iloc[0]

        # Identify blocks of consistent rows based on the most common difference
        consistent_blocks = []
        current_block = []

        for i in range(len(df)):
            if i == 0 or df.loc[i, 'time_diff'] == most_common_diff:
                current_block.append(i)
            else:
                # Save the current block and start a new one
                if current_block:
                    consistent_blocks.append(current_block)
                current_block = [i]

        # Add the last block if it exists
        if current_block:
            consistent_blocks.append(current_block)

        # Find the largest block
        largest_block = max(consistent_blocks, key=len)

        # Create cleaned DataFrame with consistent rows
        cleaned_df = df.loc[largest_block].copy()

        # Collect all indices in the largest block
        largest_block_indices = set(largest_block)

        # Separate bad entries
        invalid_entries = df.loc[~df.index.isin(largest_block_indices)].copy()

        # Drop auxiliary columns
        cleaned_df.drop(columns=['time_diff'], inplace=True)
        invalid_entries.drop(columns=['time_diff'], inplace=True)

        self.logger.info(f'Invalid Entries Count: {len(invalid_entries)}')
        self.logger.info(f'Invalid Entries: {invalid_entries}')

        # pd infer_freq required atlest 3 entries and hence checking this is necessary to avoid errors in the training process
        if len(cleaned_df) < 3:
            cleaned_df = pd.DataFrame()

        return cleaned_df

    def execute_training_pipeline(self):

        try:
            predictor = self.train()

            self.create_and_save_summary(predictor)
            model_dict = {
                "output_prediction_count": self.output_prediction_count,
                "input_context_count": self.input_context_count,
                "date_field": self.date_field,
                "group_by_cols": self.group_by_cols,
                "freq": self.freq,
                "input_columns": self.input_columns,
                "target_columns": self.target_columns,
                "output_columns": self.output_columns,
                "feature_extractor": self.feature_extractor
            }

            self.save_artifacts(model_dict, predictor)
            self.pipeline_status.update(Status.SUCCESS, "End of Training")
            self.logger.info("Training Pipeline Completed")
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


    def split_train_validation_datasets(self, data, groupby_columns, target_columns, time_stamp_column, split_ratio=0.8):
        """
        Split data into train and validation datasets while preserving item_id structure.

        Parameters:
            data (pd.DataFrame): The input dataframe from csv.
            groupby_columns (list): Columns used to define `item_id` (e.g., ["deviceName", "siteID"]).
            target_columns (list): Columns to use as targets.
            split_ratio (float): The ratio of the data to use for training (default 0.8).
            freq (str): The frequency of the time series (default "1H").

        Returns:
            train_ds (ListDataset): The training dataset.
            val_ds (ListDataset): The validation dataset.
        """
        grouped_data = data.groupby(groupby_columns)
        train_list = []
        val_list = []

        for group_key, group_df in grouped_data:
            # Sort group data by timestamp
            group_df = group_df.sort_values(time_stamp_column)
            group_df = self._remove_inconsistent_records(group_df, time_stamp_column)
            self.freq = pd.infer_freq(group_df[self.date_field])
            self.logger.info(f'Frequency for {group_key} is :: {self.freq}')

            # Calculate split index
            split_idx = int(len(group_df) * split_ratio)

            # Split into train and validation sets
            train_df = group_df.iloc[:split_idx]
            val_df = group_df.iloc[split_idx:]

            # Prepare train data
            train_target = train_df[target_columns].values.T
            train_start_time = pd.to_datetime(train_df[time_stamp_column].iloc[0], unit="s")
            train_item_id = "_".join(map(str, group_key))  # Combine keys into a single string

            train_list.append({
                FieldName.TARGET: train_target,
                FieldName.START: train_start_time,
                FieldName.ITEM_ID: train_item_id
            })

            # Prepare validation data
            val_target = val_df[target_columns].values.T
            val_start_time = pd.to_datetime(val_df[time_stamp_column].iloc[0], unit="s")
            val_item_id = "_".join(map(str, group_key))  # Combine keys into a single string
            val_list.append({
                FieldName.TARGET: val_target,
                FieldName.START: val_start_time,
                FieldName.ITEM_ID: val_item_id
            })

        self.logger.info(f"Frequency: {self.freq}")
        if 's' in self.freq:
                raise ValueError("The specified frequency 's' is not supported. Please use a valid frequency.")
        self.logger.info(f"Unique Item_ID(s): {len(train_list)}")
        result_dict = {rec['item_id']: rec['target'].T.shape[0] for rec in train_list}
        self.logger.info(f"Training Data (Item_ID & Number of Records): {result_dict}")

        result_dict = {rec['item_id']: rec['target'].T.shape[0] for rec in val_list}
        self.logger.info(f"Validation Data  (Item_ID & Number of Records): {result_dict}")

        # Create ListDatasets
        train_ds = ListDataset(data_iter=train_list, freq=self.freq, one_dim_target=False)
        val_ds = ListDataset(data_iter=val_list, freq=self.freq, one_dim_target=False)

        return train_ds, val_ds

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

            self.date_field = self.feature_extractor.timestamp
            if not self.date_field:
                raise ValueError("Timestamp field is missing or incorrectly specified in the config.json")

            self.input_columns = self.feature_extractor.get_input_features_list()
            self.logger.info(f"Input columns: {self.input_columns}")
            self.target_columns = self.feature_extractor.get_numerical_inputs_list()
            self.logger.info(f"Target columns: {self.target_columns}")
            self.output_columns = self.feature_extractor.get_output_features_list()
            self.logger.info(f"Output columns: {self.output_columns}")

            self.pipeline_status.update(Status.INPROGRESS, "Start of Training")
            
            df = pd.read_csv(full_csv_file_path, usecols=self.input_columns)
            df[self.date_field] = pd.to_datetime(df[self.date_field], unit='s')
            df.sort_values(by=self.date_field, inplace=True)

            self.group_by_cols = self.feature_extractor.get_group_by_cols()
            if not self.group_by_cols:
                raise ValueError("Group By field is missing or incorrectly specified in the config.json")

            # Prepare the datasets
            self.train_data, self.test_data = self.split_train_validation_datasets(data=df,
                                                                                   groupby_columns=self.group_by_cols,
                                                                                   target_columns=self.target_columns,
                                                                                   time_stamp_column=self.date_field)

            self.input_context_count = self.feature_extractor.input_context_count
            self.output_prediction_count = self.feature_extractor.output_prediction_count
            self.logger.info(f"Prediction length: {self.output_prediction_count}")
            if not self.output_prediction_count:
                raise ValueError("Prediction Length is missing or incorrectly specified in the config.json")

            self.logger.info("Initializing DeepVAR model")
            best_params = {"num_layers": 5, "num_cells": 61, "dropout_rate": 0.1}
            estimator = DeepVAREstimator(
                num_layers=best_params["num_layers"],
                num_cells=best_params["num_cells"],
                dropout_rate=best_params["dropout_rate"],
                target_dim=len(self.target_columns),
                prediction_length=self.output_prediction_count,
                freq=self.freq,
                trainer=Trainer(epochs=self.num_epochs)
            )

            self.logger.info("Starting model training")
            predictor = estimator.train(training_data=self.train_data)
            self.logger.info("Model training completed successfully")
            return predictor
        # except AssertionError as e:
        #     if "freq s not supported" in str(e):
        #         self.logger.error("Error: The specified frequency 's' is not supported. Please use a valid frequency.")
        #         self.pipeline_status.update(Status.FAILURE, 'Error: The specified frequency "s" is not supported. Please use a valid frequency: Valid Frequencies : ["M", "ME", "W", "D", "B", "H", "h", "min", "T"]')
        #     else:
        #         self.pipeline_status.update(Status.FAILURE, str(e))
        # except Exception as e:
        #     if "Reached maximum number of idle transformation" in str(e):
        #         self.logger.error("Error: Reached the maximum number of idle transformations. Consider optimizing your transformations or increasing limits for each transformation.")
        #         self.pipeline_status.update(Status.FAILURE, 'Error: Reached the maximum number of idle transformations. Consider optimizing your transformations or increasing limits for each transformation.')
        #     else:
        #         traceback.print_exc()
        #         self.logger.error(f"Error while training model: {e}")
        #         self.pipeline_status.update(Status.FAILURE, f"Error while training model: {e}")
        except AssertionError as e:
            if "freq s not supported" in str(e):
                self.logger.info('Error: The specified frequency "s" is not supported. Please use a valid frequency: Valid Frequencies : ["M", "ME", "W", "D", "B", "H", "h", "min", "T"]')

            raise HedgeTrainingException("Training failed due to invalid data : {e}", error_code=2001) from e
        except Exception as e:
            if "Reached maximum number of idle transformation" in str(e):
                self.logger.error("Error: Reached the maximum number of idle transformations. Consider optimizing your transformations or increasing limits for each transformation.")
                self.pipeline_status.update(Status.FAILURE, 'Error: Reached the maximum number of idle transformations. Consider optimizing your transformations or increasing limits for each transformation.')

            raise HedgeTrainingException(f"Unexpected error in train: {e}", error_code=2002) from e

    def find_optimal_params(self, dataset, output_prediction_count, freq, target_columns):
        start_time = time.time()
        study = optuna.create_study(direction="minimize", study_name="Forecasting Multi-Target Data")
        study.optimize(
            DeepVARTuningObjective(dataset, output_prediction_count, freq, target_columns), n_trials=10
        )
        trial = study.best_trial
        self.logger.info(f"Best Trial Value: {trial.value}")
        self.logger.info(f"Best Params: {trial.params}")
        self.logger.info(f"Total time to find best params: {time.time() - start_time} secs")
        return trial.params

    def create_and_save_summary(self, predictor):
        quantiles = (0.5, 0.9)
        metrics_keys = [
            "MSE", "abs_error", "abs_target_sum", "abs_target_mean", "seasonal_error",
            "MAPE", "sMAPE", "num_masked_target_values", "QuantileLoss[0.5]", "QuantileLoss[0.9]",
            "RMSE", "wQuantileLoss[0.5]", "wQuantileLoss[0.9]", "mean_absolute_QuantileLoss",
            "mean_wQuantileLoss", "MAE_Coverage"
        ]
        try:
            agg_metrics, item_metrics = backtest_metrics(
                test_dataset=self.test_data,
                predictor=predictor,
                evaluator=MultivariateEvaluator(quantiles=quantiles)
            )
            metrics = {k: agg_metrics[k] for k in metrics_keys if k in agg_metrics}
            self.logger.info(f"Evaluation Metric Summary: {metrics}")
            path_to_training_data = f"{self.data_util.base_path}/data"
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)
            asset_path = os.path.join(f"{export_path}", "assets")
            os.makedirs(asset_path, exist_ok=True)
            config_file_path = f'{path_to_training_data}/config.json'
            shutil.copy2(config_file_path, asset_path)
            summary_text = json.dumps(metrics, indent=4)
            with open(f'{asset_path}/training_summary.txt', "w") as model_summary:
                model_summary.write(summary_text)
            self.logger.info(f"Training summary saved to {asset_path}/training_summary.txt")
        except RuntimeError as e:
            raise HedgeTrainingException("Failed to generate summary", error_code=3001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in create_summary: {e}", error_code=3002) from e

    def save_artifacts(self, model_params_dict: dict, predictor_model):
        try:
            export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
            os.makedirs(export_path, exist_ok=True)
            joblib.dump(model_params_dict, filename=f'{export_path}/model_params_dict.gz')
            joblib.dump(predictor_model, filename=f'{export_path}/predictor.gz')
            os.chdir(self.model_dir)
            model_zip_file = shutil.make_archive('hedge_export', 'zip', root_dir=".", base_dir=f'./{self.algo_name}')
            self.data_util.upload_data(model_zip_file)
        except IOError as e:
            raise HedgeTrainingException("Failed to save artifacts", error_code=4001) from e
        except Exception as e:
            raise HedgeTrainingException(f"Unexpected error in save_artifacts: {e}", error_code=4002) from e


if __name__ == "__main__":
    TimeSeriesForecasting().execute_training_pipeline()
