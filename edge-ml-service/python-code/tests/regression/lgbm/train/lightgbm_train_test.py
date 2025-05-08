"""
(c) Copyright 2020-2024 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""

import unittest
import os
import json
import numpy as np
import pandas as pd
from unittest.mock import patch, MagicMock, mock_open
from io import StringIO

from regression.lightgbm.train.src.main.task import LightGBMRegression
from common.src.ml.hedge_status import HedgePipelineStatus, Status
from common.src.util.exceptions import HedgeTrainingException


class TestLightGBMRegression(unittest.TestCase):

    @patch('regression.lightgbm.train.src.main.task.EnvironmentUtil.get_env_value')
    @patch('regression.lightgbm.train.src.main.task.HedgeTrainingBase.get_env_vars')
    def test_get_env_vars(self, mock_parent_get_env_vars, mock_get_env_value):
        mock_parent_get_env_vars.return_value = None
        mock_get_env_value.side_effect = lambda key, default: {
            'LEARNING_RATE': 0.05,
            'NUM_BOOST_ROUND': 30
        }.get(key, default)

        lgbm = LightGBMRegression()
        lgbm.get_env_vars()

        self.assertEqual(lgbm.learning_rate, 0.05)
        self.assertEqual(lgbm.num_boost_round, 30)
        mock_parent_get_env_vars.assert_called_once()

    @patch('common.src.ml.hedge_training.open',
           new_callable=mock_open,
            read_data=json.dumps({
                "primaryProfile": "WindTurbine",
               "mlAlgorithm": "HedgeRegression",
               "featuresByProfile": {
                   "WindTurbine": [
                        {"type": "METRIC", "name": "feature1", "isInput": True, "isOutput": False},
                        {"type": "METRIC", "name": "feature2", "isInput": True, "isOutput": False},
                        {"type": "METRIC", "name": "target", "isInput": False, "isOutput": True}
                    ]
               },
                "featureNameToColumnIndex": {
                    "WindTurbine#feature1": 0,
                    "WindTurbine#feature2": 1,
                   "WindTurbine#target": 2
               }
           }))

    @patch('regression.lightgbm.train.src.main.task.os.path.exists', return_value=True)
    @patch('regression.lightgbm.train.src.main.task.DataFramePreprocessor')
    @patch('regression.lightgbm.train.src.main.task.pd.read_csv')
    @patch('regression.lightgbm.train.src.main.task.train_test_split')
    @patch('regression.lightgbm.train.src.main.task.lgb.train')
    @patch('lightgbm.Dataset')
    def test_train(self, mock_lgb_dataset, mock_lgb_train, mock_train_test_split, mock_read_csv,
                   mock_dataframe_preprocessor, mock_path_exists, mock_open_file):
        # Setup the mock DataFrame and transformations
        df = pd.DataFrame({
            'WindTurbine#feature1': np.random.rand(100),
            'WindTurbine#feature2': np.random.rand(100),
            'WindTurbine#target': np.random.rand(100) * 10
        })
        mock_read_csv.return_value = df

        mock_train_test_split.return_value = (df.iloc[:80], df.iloc[80:])

        mock_preprocessor_instance = MagicMock()
        mock_dataframe_preprocessor.return_value = mock_preprocessor_instance
        mock_preprocessor_instance.fit_transform.return_value = df.iloc[:80][
            ["WindTurbine#feature1", "WindTurbine#feature2"]
        ]
        mock_preprocessor_instance.transform.return_value = df.iloc[80:][
            ["WindTurbine#feature1", "WindTurbine#feature2"]
        ]

        mock_model = MagicMock()
        mock_lgb_train.return_value = mock_model
        mock_model.predict.return_value = np.random.rand(20)

        mock_dataset_instance = MagicMock()
        mock_lgb_dataset.return_value = mock_dataset_instance

        lgbm = LightGBMRegression()
        lgbm.data_util = MagicMock()
        lgbm.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/config.json",
            csv_file_path="/mock/path/data.csv"
        )
        lgbm.pipeline_status = HedgePipelineStatus()
        lgbm.train()

        mock_read_csv.assert_called_once_with("/mock/path/data.csv")
        mock_preprocessor_instance.fit_transform.assert_called_once()
        mock_preprocessor_instance.transform.assert_called_once()

        self.assertTrue(mock_lgb_dataset.call_count >= 2)
        mock_lgb_train.assert_called_once()

        self.assertIn('rmse', lgbm.artifact_dictionary)
        self.assertIn('relative_rmse', lgbm.artifact_dictionary)
        self.assertIsNotNone(lgbm.model)

    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.joblib.dump')
    @patch('regression.lightgbm.train.src.main.task.shutil.make_archive')
    @patch('regression.lightgbm.train.src.main.task.os.chdir')
    def test_save_artifacts(self, mock_chdir, mock_make_archive, mock_joblib_dump, mock_makedirs):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.model_dir = "/mock/job/dir"
        lgbm.algo_name = "mock_algo"
        lgbm.data_util.upload_data = MagicMock()

        artifact_dict = {"transformer": MagicMock()}
        model = MagicMock()

        lgbm.save_artifacts(artifact_dict, model)

        mock_makedirs.assert_called_with('/mock/base/path/hedge_export', exist_ok=True)
        self.assertEqual(mock_joblib_dump.call_count, 2)
        mock_chdir.assert_called_with('/mock/job/dir')
        mock_make_archive.assert_called_once()
        lgbm.data_util.upload_data.assert_called_once()

    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.joblib.dump')
    @patch('regression.lightgbm.train.src.main.task.shutil.make_archive')
    @patch('regression.lightgbm.train.src.main.task.os.chdir')
    def test_save_artifacts(self, mock_chdir, mock_make_archive, mock_joblib_dump, mock_makedirs):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.model_dir = "/mock/job/dir"
        lgbm.algo_name = "mock_algo"
        lgbm.data_util.upload_data = MagicMock()

        artifact_dict = {"transformer": MagicMock()}
        model = MagicMock()

        lgbm.save_artifacts(artifact_dict, model)

        mock_makedirs.assert_called_with('/mock/base/path/hedge_export', exist_ok=True)
        self.assertEqual(mock_joblib_dump.call_count, 2)
        mock_chdir.assert_called_with('/mock/job/dir')
        mock_make_archive.assert_called_once()
        lgbm.data_util.upload_data.assert_called_once()

    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.shutil.copy2')
    def test_create_and_save_summary(self, mock_copy2, mock_makedirs):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.artifact_dictionary = {
            'rmse': 1.23,
            'relative_rmse': 12.3
        }

        m = mock_open()
        with patch('builtins.open', m, create=True):
            lgbm.create_and_save_summary()

        mock_makedirs.assert_called()
        mock_copy2.assert_called()

        handle = m()
        written_content = "".join(call.args[0] for call in handle.write.mock_calls)
        self.assertIn("Validation RMSE: 1.23", written_content)
        self.assertIn("Relative RMSE: 12.3%", written_content)

    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.get_env_vars')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.train')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.create_and_save_summary')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.save_artifacts')
    def test_execute_training_pipeline(self, mock_save_artifacts, mock_create_summary, mock_train, mock_get_env_vars):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()

        lgbm.pipeline_status = MagicMock()
        lgbm.pipeline_status.is_success = True
        lgbm.pipeline_status.message = ""

        lgbm.execute_training_pipeline()

        mock_get_env_vars.assert_called_once()
        mock_train.assert_called_once()
        mock_create_summary.assert_called_once()
        mock_save_artifacts.assert_called_once()
        lgbm.data_util.update_status.assert_called_with(True, "")

    @patch('regression.lightgbm.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars_key_error(self, mock_get_env_value):
        lgbm = LightGBMRegression()
        mock_get_env_value.side_effect = KeyError("LEARNING_RATE not found")

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.get_env_vars()

        self.assertIn("Environment variable is missing", str(context.exception))
        self.assertEqual(context.exception.error_code, 1001)

    @patch('regression.lightgbm.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars_key_error(self, mock_get_env_value):
        lgbm = LightGBMRegression()
        mock_get_env_value.side_effect = ValueError("Unexpected error")

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.get_env_vars()

        self.assertIn("Unexpected error in get_env_vars:", str(context.exception))
        self.assertEqual(context.exception.error_code, 1002)

    def test_train_data_util_none(self):
        lgbm = LightGBMRegression()
        lgbm.data_util = None
        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.train()
        self.assertIn("Training failed due to invalid data", str(context.exception))
        self.assertEqual(context.exception.error_code, 2001)

    @patch('common.src.ml.hedge_training.open', new_callable=mock_open,
           read_data=json.dumps({
               "primaryProfile": "WindTurbine",
               "featuresByProfile": {"WindTurbine": []}
           }))
    @patch('regression.lightgbm.train.src.main.task.os.path.exists', return_value=True)
    @patch('regression.lightgbm.train.src.main.task.pd.read_csv')
    @patch('regression.lightgbm.train.src.main.task.FeatureExtractor')
    def test_train_no_target_column(self, mock_feature_extractor, mock_read_csv, mock_path_exists, mock_open_file):
        df = pd.DataFrame({'col1': [1, 2, 3]})
        mock_read_csv.return_value = df

        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor.return_value = mock_feature_extractor_instance
        mock_feature_extractor_instance.get_numerical_inputs_list.return_value = ["col1"]
        mock_feature_extractor_instance.get_numerical_outputs_list.return_value = [""]

        lgbm = LightGBMRegression()
        lgbm.data_util = MagicMock()
        lgbm.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/config.json",
            csv_file_path="/mock/path/data.csv"
        )

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.train()

        self.assertIn("Training failed due to invalid data", str(context.exception))
        self.assertEqual(context.exception.error_code, 2001)

    @patch('common.src.ml.hedge_training.open', new_callable=mock_open, read_data=json.dumps({
        "primaryProfile": "WindTurbine",
        "mlAlgorithm": "HedgeRegression",
        "featuresByProfile": {
            "WindTurbine": [
                {"type": "METRIC", "name": "feature1", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "feature2", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "target", "isInput": False, "isOutput": True}
            ]
        },
        "featureNameToColumnIndex": {
            "WindTurbine#feature1": 0,
            "WindTurbine#feature2": 1,
            "WindTurbine#target": 2
        }
    }))
    @patch('regression.lightgbm.train.src.main.task.os.path.exists', return_value=True)
    @patch('regression.lightgbm.train.src.main.task.pd.read_csv')
    def test_train_unexpected_exception(self, mock_read_csv, mock_path_exists, mock_open_file):
        mock_read_csv.side_effect = Exception("Unexpected CSV read error")

        lgbm = LightGBMRegression()
        lgbm.data_util = MagicMock()
        lgbm.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.train()
        self.assertIn("Unexpected error in train:", str(context.exception))
        self.assertEqual(context.exception.error_code, 2002)

    @patch('regression.lightgbm.train.src.main.task.joblib.dump')
    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.os.chdir')
    @patch('regression.lightgbm.train.src.main.task.shutil.make_archive')
    def test_save_artifacts_io_error(self, mock_make_archive, mock_chdir, mock_makedirs, mock_joblib_dump):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.model_dir = "/mock/job/dir"
        lgbm.algo_name = "mock_algo"
        lgbm.data_util.upload_data = MagicMock()

        mock_joblib_dump.side_effect = IOError("Failed to write file")

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.save_artifacts({"key": "value"}, MagicMock())
        self.assertIn("Failed to save artifacts", str(context.exception))
        self.assertEqual(context.exception.error_code, 4001)

    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.train')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.create_and_save_summary')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.save_artifacts')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.get_env_vars')
    def test_execute_training_pipeline_train_failure(self, mock_get_env_vars, mock_save_artifacts, mock_create_summary,
                                                     mock_train):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.pipeline_status = MagicMock()

        def update_side_effect(status, message):
            if status == Status.FAILURE:
                lgbm.pipeline_status.is_success = False
            else:
                lgbm.pipeline_status.is_success = True

            lgbm.pipeline_status.message = message

        lgbm.pipeline_status.update.side_effect = update_side_effect

        mock_train.side_effect = HedgeTrainingException("Simulated training failure", error_code=2001)

        lgbm.execute_training_pipeline()

        lgbm.pipeline_status.update.assert_called_once()
        called_status, called_message = lgbm.pipeline_status.update.call_args[0]

        self.assertEqual(called_status, Status.FAILURE)
        self.assertIn("Simulated training failure", called_message)

        lgbm.data_util.update_status.assert_called_once_with(False, called_message)

    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.train')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.create_and_save_summary')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.save_artifacts')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.get_env_vars')
    def test_execute_training_pipeline_save_artifacts_failure(self, mock_get_env_vars, mock_save_artifacts,
                                                              mock_create_summary, mock_train):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.pipeline_status = MagicMock()

        def update_side_effect(status, message):
            if status == Status.FAILURE:
                lgbm.pipeline_status.is_success = False
            else:
                lgbm.pipeline_status.is_success = True
            lgbm.pipeline_status.message = message

        lgbm.pipeline_status.update.side_effect = update_side_effect

        mock_save_artifacts.side_effect = HedgeTrainingException("Simulated artifact save failure", error_code=4001)

        lgbm.execute_training_pipeline()

        lgbm.pipeline_status.update.assert_called_once()
        called_status, called_message = lgbm.pipeline_status.update.call_args[0]

        self.assertEqual(called_status, Status.FAILURE)
        self.assertIn("Simulated artifact save failure", called_message)
        lgbm.data_util.update_status.assert_called_once_with(False, called_message)

    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.shutil.copy2')
    def test_create_and_save_summary_unexpected_exception(self, mock_copy2, mock_makedirs):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.artifact_dictionary = {
            'rmse': 1.23,
            'relative_rmse': 12.3
        }

        mock_copy2.side_effect = Exception("Generic exception")

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.create_and_save_summary()

        self.assertIn("Unexpected error in create_summary:", str(context.exception))
        self.assertEqual(context.exception.error_code, 3002)

    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.joblib.dump')
    @patch('regression.lightgbm.train.src.main.task.shutil.make_archive')
    @patch('regression.lightgbm.train.src.main.task.os.chdir')
    def test_save_artifacts_unexpected_exception(self, mock_chdir, mock_make_archive, mock_joblib_dump, mock_makedirs):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.model_dir = "/mock/job/dir"
        lgbm.algo_name = "mock_algo"
        lgbm.data_util.upload_data = MagicMock()

        artifact_dict = {"transformer": MagicMock()}
        model = MagicMock()

        mock_joblib_dump.side_effect = ValueError("Some unexpected error occurred")

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.save_artifacts(artifact_dict, model)

        self.assertIn("Unexpected error in save_artifacts:", str(context.exception))
        self.assertEqual(context.exception.error_code, 4002)

    @patch('common.src.ml.hedge_training.open', new_callable=mock_open, read_data=json.dumps({
        "primaryProfile": "WindTurbine",
        "mlAlgorithm": "HedgeRegression",
        "featuresByProfile": {
            "WindTurbine": [
                {"type": "METRIC", "name": "feature1", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "feature2", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "target", "isInput": True, "isOutput": True}
            ]
        },
        "featureNameToColumnIndex": {
            "WindTurbine#feature1": 0,
            "WindTurbine#feature2": 1,
            "WindTurbine#target": 2
        }
    }))
    @patch('regression.lightgbm.train.src.main.task.os.path.exists', return_value=True)
    @patch('regression.lightgbm.train.src.main.task.DataFramePreprocessor')
    @patch('regression.lightgbm.train.src.main.task.pd.read_csv')
    @patch('regression.lightgbm.train.src.main.task.train_test_split')
    @patch('regression.lightgbm.train.src.main.task.lgb.train')
    @patch('regression.lightgbm.train.src.main.task.lgb.Dataset')
    def test_target_column_in_input_columns(self, mock_lgb_dataset, mock_lgb_train, mock_train_test_split,
                                            mock_read_csv, mock_dataframe_preprocessor, mock_path_exists,
                                            mock_open_file):
        df = pd.DataFrame({
            'WindTurbine#feature1': np.random.rand(100),
            'WindTurbine#feature2': np.random.rand(100),
            'WindTurbine#target': np.random.rand(100) * 10
        })
        mock_read_csv.return_value = df

        mock_train_test_split.return_value = (df.iloc[:80], df.iloc[80:])

        mock_preprocessor_instance = MagicMock()
        mock_dataframe_preprocessor.return_value = mock_preprocessor_instance
        mock_preprocessor_instance.fit_transform.return_value = df.iloc[:80][
            ["WindTurbine#feature1", "WindTurbine#feature2"]]
        mock_preprocessor_instance.transform.return_value = df.iloc[80:][
            ["WindTurbine#feature1", "WindTurbine#feature2"]]

        mock_model = MagicMock()
        mock_lgb_train.return_value = mock_model
        mock_model.predict.return_value = np.random.rand(20)

        mock_dataset_instance = MagicMock()
        mock_lgb_dataset.return_value = mock_dataset_instance

        lgbm = LightGBMRegression()
        lgbm.data_util = MagicMock()
        lgbm.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/config.json",
            csv_file_path="/mock/path/data.csv"
        )
        lgbm.pipeline_status = HedgePipelineStatus()
        lgbm.train()

        called_args_fit = mock_preprocessor_instance.fit_transform.call_args[0][0].columns.tolist()
        self.assertNotIn('WindTurbine#target', called_args_fit,
                         "Target column should have been removed from input columns")

        called_args_transform = mock_preprocessor_instance.transform.call_args[0][0].columns.tolist()
        self.assertNotIn('WindTurbine#target', called_args_transform,
                         "Target column should have been removed from input columns")

    @patch('common.src.ml.hedge_training.open', new_callable=mock_open, read_data=json.dumps({
        "primaryProfile": "WindTurbine",
        "mlAlgorithm": "HedgeRegression",
        "featuresByProfile": {
            "WindTurbine": [
                {"type": "METRIC", "name": "feature1", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "feature2", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "target", "isInput": False, "isOutput": True}
            ]
        },
        "featureNameToColumnIndex": {
            "WindTurbine#feature1": 0,
            "WindTurbine#feature2": 1,
            "WindTurbine#target": 2
        }
    }))
    @patch('regression.lightgbm.train.src.main.task.os.path.exists', return_value=True)
    @patch('regression.lightgbm.train.src.main.task.DataFramePreprocessor')
    @patch('regression.lightgbm.train.src.main.task.pd.read_csv')
    @patch('regression.lightgbm.train.src.main.task.train_test_split')
    @patch('regression.lightgbm.train.src.main.task.lgb.train')
    @patch('regression.lightgbm.train.src.main.task.lgb.Dataset')
    def test_mean_actual_zero_or_negative(self, mock_lgb_dataset, mock_lgb_train, mock_train_test_split,
                                          mock_read_csv, mock_dataframe_preprocessor, mock_path_exists, mock_open_file):
        df = pd.DataFrame({
            'WindTurbine#feature1': np.random.rand(100),
            'WindTurbine#feature2': np.random.rand(100),
            'WindTurbine#target': np.zeros(100)  # All zeros ensures mean_actual will be 0 after inverse scaling
        })
        mock_read_csv.return_value = df

        mock_train_test_split.return_value = (df.iloc[:80], df.iloc[80:])

        mock_preprocessor_instance = MagicMock()
        mock_dataframe_preprocessor.return_value = mock_preprocessor_instance
        mock_preprocessor_instance.fit_transform.return_value = df.iloc[:80][
            ["WindTurbine#feature1", "WindTurbine#feature2"]]
        mock_preprocessor_instance.transform.return_value = df.iloc[80:][
            ["WindTurbine#feature1", "WindTurbine#feature2"]]

        mock_model = MagicMock()
        mock_lgb_train.return_value = mock_model
        mock_model.predict.return_value = np.zeros(20)

        mock_dataset_instance = MagicMock()
        mock_lgb_dataset.return_value = mock_dataset_instance

        lgbm = LightGBMRegression()
        lgbm.data_util = MagicMock()
        lgbm.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/config.json",
            csv_file_path="/mock/path/data.csv"
        )
        lgbm.pipeline_status = HedgePipelineStatus()
        lgbm.logger = MagicMock()

        lgbm.train()

        lgbm.logger.warning.assert_any_call("Mean of actual values is zero or negative, Relative RMSE is undefined.")

        self.assertTrue(np.isinf(lgbm.artifact_dictionary['relative_rmse']))

    @patch('regression.lightgbm.train.src.main.task.os.makedirs')
    @patch('regression.lightgbm.train.src.main.task.shutil.copy2', side_effect=RuntimeError("Mocked runtime error"))
    def test_create_and_save_summary_runtime_error(self, mock_copy2, mock_makedirs):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()
        lgbm.data_util.base_path = "/mock/base/path"
        lgbm.artifact_dictionary = {
            'rmse': 1.23,
            'relative_rmse': 12.3
        }

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.create_and_save_summary()

        self.assertIn("Failed to generate summary", str(context.exception))
        self.assertEqual(context.exception.error_code, 3001)

    @patch('regression.lightgbm.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars_key_error(self, mock_get_env_value):
        lgbm = LightGBMRegression()
        mock_get_env_value.side_effect = KeyError("Missing environment variable")

        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.get_env_vars()

        self.assertIn("Environment variable is missing", str(context.exception))
        self.assertEqual(context.exception.error_code, 1001)

    @patch('regression.lightgbm.train.src.main.task.EnvironmentUtil.get_env_value',
           side_effect=ValueError("Unexpected error"))
    def test_get_env_vars_unexpected_exception(self, mock_get_env_value):
        lgbm = LightGBMRegression()
        # This should trigger the "except Exception" block in get_env_vars
        with self.assertRaises(HedgeTrainingException) as context:
            lgbm.get_env_vars()

        self.assertIn("Unexpected error in get_env_vars:", str(context.exception))
        self.assertEqual(context.exception.error_code, 1002)

    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.get_env_vars')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.train',
           side_effect=ValueError("Some unexpected error"))
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.create_and_save_summary')
    @patch('regression.lightgbm.train.src.main.task.LightGBMRegression.save_artifacts')
    def test_execute_training_pipeline_unexpected_exception(self, mock_save_artifacts, mock_create_summary, mock_train,
                                                            mock_get_env_vars):
        lgbm = LightGBMRegression()
        lgbm.logger = MagicMock()
        lgbm.data_util = MagicMock()

        lgbm.pipeline_status = MagicMock()

        def update_side_effect(status, message):
            lgbm.pipeline_status.is_success = (status != Status.FAILURE)
            lgbm.pipeline_status.message = message

        lgbm.pipeline_status.update.side_effect = update_side_effect

        lgbm.execute_training_pipeline()

        lgbm.logger.error.assert_any_call("An unexpected error occurred: Some unexpected error")
        self.assertGreater(lgbm.logger.error.call_count, 1)

        lgbm.pipeline_status.update.assert_called_once()
        args, kwargs = lgbm.pipeline_status.update.call_args
        self.assertEqual(args[0], Status.FAILURE)
        self.assertIn("An unexpected error occurred: Some unexpected error", args[1])

        self.assertFalse(lgbm.pipeline_status.is_success)
        self.assertIn("An unexpected error occurred: Some unexpected error", lgbm.pipeline_status.message)

        lgbm.data_util.update_status.assert_called_once_with(False, lgbm.pipeline_status.message)


if __name__ == '__main__':
    unittest.main()
