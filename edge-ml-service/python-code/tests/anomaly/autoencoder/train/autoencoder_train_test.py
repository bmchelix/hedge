"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import sys
import os
import unittest
import shutil
import json
from unittest.mock import patch, MagicMock, mock_open
import pandas as pd
import numpy as np
from anomaly.autoencoder.train.src.main.task import Autoencoder
from common.src.ml.hedge_status import HedgePipelineStatus, Status
from common.src.util.exceptions import HedgeTrainingException


FAILURE: str = "FAILURE"

class TestAutoencoderTraining(unittest.TestCase):
    @patch('common.src.ml.hedge_training.open', new_callable=mock_open, read_data=json.dumps({
        "name": "bam-bam",
        "description": "gillian is backkkkk at it",
        "mlAlgorithm": "HedgeAnomaly",
        "primaryProfile": "WindTurbine",
        "featuresByProfile": {
            "WindTurbine": [
                {"type": "METRIC", "name": "WindSpeed", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "TurbinePower", "isInput": True, "isOutput": False},
                {"type": "METRIC", "name": "RotorSpeed", "isInput": True, "isOutput": False},
            ]
        },
        "groupByMetaData": ["deviceName"],
        "trainingDataFilters": [
            {"label": "deviceName", "operator": "CONTAINS", "value": "AltaNS14"}
        ],
        "samplingIntervalSecs": 30,
        "trainingDurationSecs": 864000,
        "inputTopicForInferencing": "hedge/events/device/WindTurbine/#",
        "enabled": True,
        "modelVersion": 1,
        "featureNameToColumnIndex": {
            "WindTurbine#RotorSpeed": 2,
            "WindTurbine#TurbinePower": 1,
            "WindTurbine#WindSpeed": 0
        }
    }))
    @patch('anomaly.autoencoder.train.src.main.task.os.path.exists', return_value=True)
    @patch('anomaly.autoencoder.train.src.main.task.DataFramePreprocessor')
    @patch('anomaly.autoencoder.train.src.main.task.pd.read_csv')
    @patch('anomaly.autoencoder.train.src.main.task.train_test_split')
    @patch('anomaly.autoencoder.train.src.main.task.tf.keras.Model.fit')
    @patch('anomaly.autoencoder.train.src.main.task.tf.keras.Model.predict')
    @patch('anomaly.autoencoder.train.src.main.task.tf.keras.Model.evaluate')
    def test_train(self, mock_evaluate, mock_predict, mock_keras_fit, mock_train_test_split,
                   mock_read_csv, mock_dataframe_preprocessor, mock_path_exists, mock_open_file):
        mock_df = pd.DataFrame(np.random.rand(100, 10), columns=[f'col_{i}' for i in range(10)])
        mock_read_csv.return_value = mock_df

        mock_train_test_split.return_value = (mock_df.iloc[:80, :], mock_df.iloc[80:, :])

        mock_preprocessor_instance = MagicMock()
        mock_dataframe_preprocessor.return_value = mock_preprocessor_instance
        mock_preprocessor_instance.fit_transform.return_value = mock_df.iloc[:80, :]
        mock_preprocessor_instance.transform.return_value = mock_df.iloc[80:, :]

        mock_keras_fit.return_value = MagicMock(history={'loss': [0.1, 0.05]})

        mock_predict.return_value = np.random.rand(20, 10)

        mock_evaluate.return_value = 0.1

        autoencoder = Autoencoder()
        autoencoder.pipeline_status = HedgePipelineStatus()

        autoencoder.data_util = MagicMock()
        autoencoder.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/to/config.json",
            csv_file_path="/mock/path/to/data.csv"
        )

        autoencoder.train()

        mock_read_csv.assert_called_once()
        mock_preprocessor_instance.fit_transform.assert_called_once_with(unittest.mock.ANY)
        mock_preprocessor_instance.transform.assert_called_once_with(unittest.mock.ANY)
        mock_keras_fit.assert_called_once()
        mock_predict.assert_called_once_with(unittest.mock.ANY)
        mock_evaluate.assert_called_once()

        self.assertIn('transformer', autoencoder.artifact_dictionary)
        self.assertIn('scaler', autoencoder.artifact_dictionary)
        self.assertIn('threshold', autoencoder.artifact_dictionary)
        self.assertEqual(autoencoder.artifact_dictionary['threshold'], autoencoder.threshold)
        self.assertTrue(hasattr(autoencoder, 'mse'))
        self.assertTrue(hasattr(autoencoder, 'mse_median'))
        self.assertTrue(hasattr(autoencoder, 'absolute_deviation'))
        self.assertTrue(hasattr(autoencoder, 'median_absolute_deviation'))


    @patch('anomaly.autoencoder.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars(self, mock_get_env_value):
        autoencoder = Autoencoder()
        # TRAINING_FILE_ID & MODELDIR to be removed sice not applicable
        mock_get_env_value.side_effect = {
            'LOCAL': 'true',
            'MODELDIR': '/mock/job/dir',
            'JOB_DIR': '/mock/job/dir',
            'TRAINING_FILE_ID': 'mock_training_file_id',
            'OUTPUTDIR': '/mock/output/dir',
            'BATCH_SIZE': 64,
            'NUM_EPOCH': 30
        }.get

        autoencoder.get_env_vars()

        self.assertEqual(autoencoder.local, 'true')
        self.assertEqual(autoencoder.model_dir, '/mock/job/dir')
        self.assertEqual(autoencoder.training_file_id, 'mock_training_file_id')
        self.assertEqual(autoencoder.data_util.base_path, '/mock/output/dir')
        self.assertEqual(autoencoder.batch_size, 64)
        self.assertEqual(autoencoder.num_epochs, 30)

        self.assertIsNotNone(autoencoder.data_util)

    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    @patch('anomaly.autoencoder.train.src.main.task.shutil.make_archive')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.save_artifacts')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.train')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.get_env_vars')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.create_and_save_summary')
    def test_execute_training_pipeline(self, mock_create_and_save_summary, mock_get_env_vars, mock_train,
                                       mock_save_artifacts, mock_make_archive, mock_os_makedirs):

        autoencoder = Autoencoder()

        autoencoder.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS)


        autoencoder.data_util = MagicMock()
        autoencoder.model = MagicMock()
        autoencoder.job_dir = "/mock/job/dir"
        autoencoder.algo_name = "mock_algo"

        autoencoder.artifact_dictionary = {
            'transformer': MagicMock(),
            'scaler': MagicMock(),
            'threshold': 2,
            'mse_median': 0.1,
            'median_absolute_deviation': 0.01
        }
        def train_side_effect():
            autoencoder.mse_median = 0.1
            autoencoder.median_absolute_deviation = 0.01

        mock_train.side_effect = train_side_effect

        def mock_save_artifacts_side_effect(artifact_dict, model):
            print(f"save_artifacts called with: {artifact_dict}, model: {model}")
            shutil.make_archive('/mock/job/dir/hedge_export', 'zip', root_dir="/anomaly/autoencoder/train/src/main",
                                base_dir=f'./{autoencoder.algo_name}')

        mock_save_artifacts.side_effect = mock_save_artifacts_side_effect

        autoencoder.execute_training_pipeline()

        mock_get_env_vars.assert_called_once()
        mock_train.assert_called_once()
        mock_create_and_save_summary.assert_called_once()
        mock_save_artifacts.assert_called_once()

        print(f"make_archive called: {mock_make_archive.called}")

        mock_make_archive.assert_called_once()

    @patch('anomaly.autoencoder.train.src.main.task.shutil.copy2')
    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    @patch('builtins.open', new_callable=mock_open)
    def test_create_and_save_summary(self, mock_open_file, mock_os_makedirs, mock_copy2):
        autoencoder = Autoencoder()
        autoencoder.num_of_layers = 5
        autoencoder.num_of_neurons = 100
        autoencoder.num_epochs_run = 10
        autoencoder.loss_value = 0.01
        autoencoder.mse = np.array([0.01, 0.02])
        autoencoder.mse_median = 0.015
        autoencoder.absolute_deviation = np.array([0.005, 0.005])
        autoencoder.median_absolute_deviation = 0.005
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.base_path = '/mock/path'

        autoencoder.create_and_save_summary()

        mock_open_file.assert_called_with('/mock/path/hedge_export/assets/training_summary.txt', 'w')

    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    @patch('anomaly.autoencoder.train.src.main.task.os.chdir')
    @patch('anomaly.autoencoder.train.src.main.task.shutil.make_archive')
    @patch('anomaly.autoencoder.train.src.main.task.joblib.dump')
    def test_save_artifacts_exception(self, mock_joblib_dump, mock_make_archive, mock_os_chdir, mock_os_makedirs):
        mock_joblib_dump.side_effect = Exception("Mocked exception during joblib.dump")

        autoencoder = Autoencoder()
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.base_path = "/mock/path"
        autoencoder.model_dir = "/mock/job/dir"
        autoencoder.algo_name = "mock_algo"

        artifact_dict = {'key': 'value'}
        model = MagicMock()

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.save_artifacts(artifact_dict, model)

        self.assertIn("Unexpected error in save_artifacts", str(context.exception))

    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    def test_save_artifacts_data_util_none(self, mock_os_makedirs):
        autoencoder = Autoencoder()
        autoencoder.logger = MagicMock()
        autoencoder.data_util = None

        artifact_dict = {'key': 'value'}
        model = MagicMock()

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.save_artifacts(artifact_dict, model)

        self.assertIn("Unexpected error in save_artifacts", str(context.exception))
        mock_os_makedirs.assert_not_called()

    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.save_artifacts')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.create_and_save_summary')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.train')
    def test_execute_training_pipeline_retries_exceeded(self, mock_train, mock_create_and_save_summary,
                                                        mock_save_artifacts):
        autoencoder = Autoencoder()
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.update_status = MagicMock()
        autoencoder.data_util.base_path = '/mock/base/path'
        autoencoder.get_env_vars = MagicMock()
        autoencoder.model = MagicMock()
        autoencoder.artifact_dictionary = {}

        autoencoder.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")

        max_retries = 3

        def train_side_effect():
            autoencoder.mse_median = 0
            autoencoder.median_absolute_deviation = 0

        mock_train.side_effect = train_side_effect

        autoencoder.execute_training_pipeline(max_retries=max_retries)

        self.assertEqual(mock_train.call_count, max_retries + 1)

        expected_warning_calls = [
            unittest.mock.call(
                f"mse_median or median_absolute_deviation is zero. Retrying training ({i + 1}/{max_retries})..."
            )
            for i in range(max_retries)
        ]
        autoencoder.logger.warning.assert_has_calls(expected_warning_calls, any_order=False)

        self.assertGreaterEqual(autoencoder.logger.error.call_count, 1)
        autoencoder.logger.error.assert_any_call(
            "Training failed: Reached max retries with zero mse_median or median_absolute_deviation."
        )

    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    @patch('anomaly.autoencoder.train.src.main.task.os.chdir')
    @patch('anomaly.autoencoder.train.src.main.task.shutil.make_archive', return_value='/mock/job/dir/hedge_export.zip')
    @patch('anomaly.autoencoder.train.src.main.task.joblib.dump')
    def test_save_artifacts_success(self, mock_joblib_dump, mock_make_archive, mock_os_chdir, mock_os_makedirs):
        autoencoder = Autoencoder()
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.base_path = "/mock/path"
        autoencoder.model_dir = "/mock/job/dir"
        autoencoder.algo_name = "mock_algo"
        autoencoder.data_util.upload_data = MagicMock()

        artifact_dict = {
            'transformer': MagicMock(),
            'scaler': MagicMock(),
            'threshold': 2,
            'mse_median': 0.1,
            'median_absolute_deviation': 0.01
        }
        model = MagicMock()

        autoencoder.save_artifacts(artifact_dict, model)

        export_path = "/mock/path/hedge_export"
        mock_os_makedirs.assert_any_call(export_path, exist_ok=True)

        mock_joblib_dump.assert_any_call(artifact_dict, f"{export_path}/artifacts.gz", compress=True)
        autoencoder.logger.info.assert_any_call(
            f"Artifact dictionary saved successfully at: {export_path}/artifacts.gz")

        mock_joblib_dump.assert_any_call(model, f"{export_path}/model.gz", compress=True)
        autoencoder.logger.info.assert_any_call(f"Model saved successfully at: {export_path}/model.gz")

        mock_os_chdir.assert_called_once_with("/mock/job/dir")
        mock_make_archive.assert_called_once_with("hedge_export", "zip", root_dir=".", base_dir="./mock_algo")

        autoencoder.data_util.upload_data.assert_called_once_with("/mock/job/dir/hedge_export.zip")

    @patch('anomaly.autoencoder.train.src.main.task.FeatureExtractor')
    def test_train_value_error(self, mock_feature_extractor):
        autoencoder = Autoencoder()
        autoencoder.data_util = None

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.train()
        exception_message = str(context.exception)
        self.assertIn("Training failed due to invalid data", exception_message)
        self.assertEqual(context.exception.error_code, 2001)

    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.load_json', return_value={
        "featureNameToColumnIndex": {}
    })
    @patch('anomaly.autoencoder.train.src.main.task.pd.read_csv')
    @patch('anomaly.autoencoder.train.src.main.task.FeatureExtractor')
    def test_train_key_error(self, mock_feature_extractor, mock_read_csv, mock_load_json):
        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor_instance.get_data_object.return_value = {"featureNameToColumnIndex": {}}
        mock_feature_extractor.return_value = mock_feature_extractor_instance

        autoencoder = Autoencoder()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_read_csv.return_value = MagicMock()

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.train()
        exception_message = str(context.exception)
        self.assertIn("Training failed due to missing features", exception_message)
        self.assertEqual(context.exception.error_code, 2003)

    @patch('anomaly.autoencoder.train.src.main.task.FeatureExtractor')
    @patch('anomaly.autoencoder.train.src.main.task.pd.read_csv')
    def test_train_unexpected_exception(self, mock_read_csv, mock_feature_extractor):
        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor_instance.get_data_object.return_value = {
            "featureNameToColumnIndex": {"WindTurbine#WindSpeed": 0}
        }
        mock_feature_extractor.return_value = mock_feature_extractor_instance

        autoencoder = Autoencoder()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_read_csv.side_effect = Exception("Some unexpected error")

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.train()
        exception_message = str(context.exception)
        self.assertIn("Unexpected error in train:", exception_message)
        self.assertEqual(context.exception.error_code, 2002)

    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    @patch('anomaly.autoencoder.train.src.main.task.shutil.copy2', side_effect=RuntimeError("Mocked RuntimeError"))
    def test_create_and_save_summary_runtime_error(self, mock_copy2, mock_makedirs):
        autoencoder = Autoencoder()
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.base_path = '/mock/path'

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.create_and_save_summary()

        exception_message = str(context.exception)
        self.assertIn("Failed to generate summary", exception_message)
        self.assertEqual(context.exception.error_code, 3001)

    @patch('anomaly.autoencoder.train.src.main.task.os.makedirs')
    @patch('anomaly.autoencoder.train.src.main.task.shutil.copy2')
    def test_create_and_save_summary_unexpected_exception(self, mock_copy2, mock_makedirs):
        autoencoder = Autoencoder()
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()
        autoencoder.data_util.base_path = '/mock/path'

        with patch('builtins.open', new_callable=mock_open) as mock_open_file:
            mock_open_file.side_effect = Exception("Mocked generic exception")

            with self.assertRaises(HedgeTrainingException) as context:
                autoencoder.create_and_save_summary()

            exception_message = str(context.exception)
            self.assertIn("Unexpected error in create_summary:", exception_message)
            self.assertEqual(context.exception.error_code, 3002)

    def test_get_env_vars_key_error(self):
        autoencoder = Autoencoder()
        autoencoder.env_util = MagicMock()

        autoencoder.env_util.get_env_value.side_effect = KeyError("BATCH_SIZE not found")

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.get_env_vars()

        exception_message = str(context.exception)
        self.assertIn("Environment variable is missing", exception_message)
        self.assertEqual(context.exception.error_code, 1001)

    def test_get_env_vars_unexpected_exception(self):
        autoencoder = Autoencoder()
        autoencoder.env_util = MagicMock()

        autoencoder.env_util.get_env_value.side_effect = lambda key, default: "not_an_integer"

        with self.assertRaises(HedgeTrainingException) as context:
            autoencoder.get_env_vars()

        exception_message = str(context.exception)
        self.assertIn("Unexpected error in get_env_vars:", exception_message)
        self.assertEqual(context.exception.error_code, 1002)

    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.train')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.get_env_vars')
    def test_execute_training_pipeline_hedge_exception(self, mock_get_env_vars, mock_train):
        autoencoder = Autoencoder()
        autoencoder.pipeline_status = HedgePipelineStatus(message="")
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()

        mock_train.side_effect = HedgeTrainingException("Simulated hedge exception", error_code=9999)

        autoencoder.execute_training_pipeline()

        autoencoder.logger.error.assert_any_call(
            "Training pipeline failed with error: [Error 9999] Simulated hedge exception"
        )

        self.assertFalse(autoencoder.pipeline_status.is_success)
        self.assertIn("Training pipeline failed with error: [Error 9999] Simulated hedge exception",
                      autoencoder.pipeline_status.message)

        autoencoder.data_util.update_status.assert_called_once_with(
            autoencoder.pipeline_status.is_success, autoencoder.pipeline_status.message
        )

    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.train')
    @patch('anomaly.autoencoder.train.src.main.task.Autoencoder.get_env_vars')
    def test_execute_training_pipeline_unexpected_exception(self, mock_get_env_vars, mock_train):
        autoencoder = Autoencoder()
        autoencoder.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        autoencoder.logger = MagicMock()
        autoencoder.data_util = MagicMock()

        mock_train.side_effect = ValueError("Some unexpected error")

        autoencoder.execute_training_pipeline()

        autoencoder.logger.error.assert_any_call(
            "An unexpected error occurred: Some unexpected error"
        )
        self.assertFalse(autoencoder.pipeline_status.is_success)
        self.assertIn("An unexpected error occurred: Some unexpected error",
                      autoencoder.pipeline_status.message)

        autoencoder.data_util.update_status.assert_called_once_with(
            autoencoder.pipeline_status.is_success, autoencoder.pipeline_status.message
        )


if __name__ == '__main__':
    unittest.main()
