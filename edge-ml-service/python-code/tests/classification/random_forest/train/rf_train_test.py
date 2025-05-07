"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import unittest
from unittest.mock import MagicMock, mock_open, patch

import numpy as np
import pandas as pd

from classification.random_forest.train.src.main.task import Classification
from common.src.ml.hedge_status import Status, HedgePipelineStatus
from common.src.util.exceptions import HedgeTrainingException

class TestClassification(unittest.TestCase):

    @patch('classification.random_forest.train.src.main.task.EnvironmentUtil.get_env_value')
    @patch('classification.random_forest.train.src.main.task.LoggerUtil')
    def test_init(self, mock_logger_util, mock_env_value):
        # MODELDIR to be removed
        mock_env_value.side_effect = {
            'LOCAL': 'True',
            'JOB_DIR': '/mock/job/dir',
            'MODELDIR': '/mock/job/dir',
            'TRAINING_FILE_ID': '/mock/training_file_id'
        }.get

        classification = Classification()
        classification.get_env_vars()

        mock_logger_util.assert_called_once()
        self.assertIsNotNone(classification.logger)
        print(classification.local)
        self.assertEqual(classification.local, 'True')
        self.assertEqual(classification.model_dir, '/mock/job/dir')
        self.assertEqual(classification.training_file_id, '/mock/training_file_id')

    @patch('classification.random_forest.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars(self, mock_get_env_value):

        classification = Classification()
        classification.logger = MagicMock()

        mock_get_env_value.side_effect = {
            'LOCAL': 'True',
            'TRAINING_FILE_ID': 'training.zip',
            'OUTPUTDIR': '/mock/output/dir',
            'MODELDIR': '/mock/model/dir',
            'NUM_TREES': 100,
            'MAX_DEPTH': 10,
            'MIN_SAMPLES_SPLIT': 2,
            'MIN_SAMPLES_LEAF': 1
        }.get

        classification.get_env_vars()

        self.assertIsNotNone(classification.data_util)
        self.assertEqual(classification.data_util.base_path, '/mock/output/dir')

        self.assertEqual(classification.num_trees, 100)
        self.assertEqual(classification.max_depth, 10)
        self.assertEqual(classification.min_samples_split, 2)
        self.assertEqual(classification.min_samples_leaf, 1)

    
    @patch('builtins.open', new_callable=mock_open)
    @patch('os.makedirs')
    @patch('shutil.copy2')
    def test_create_and_save_summary(self, mock_copy2, mock_makedirs, mock_file):
        classification = Classification()
        classification.accuracy = 0.85
        classification.classification_report = "Mock Classification Report"
        classification.data_util = MagicMock()
        classification.data_util.base_path = "/mock/base/path"
        classification.logger = MagicMock()
        
        classification.create_and_save_summary()
        
        mock_makedirs.assert_any_call('/mock/base/path/hedge_export', exist_ok=True)
        mock_makedirs.assert_any_call('/mock/base/path/hedge_export/assets', exist_ok=True)
        
        mock_copy2.assert_called_once_with('/mock/base/path/data/config.json', '/mock/base/path/hedge_export/assets')
        
        # Check that open was called twice
        self.assertEqual(mock_file.call_count, 2)
        
        expected_content = (
            "Training Statistics:\n"
            "Accuracy: 0.85\n"
            "Classification Report:\nMock Classification Report\n"
        )
        mock_file().write.assert_called_once_with(expected_content)
        
        # Check that logger.info was called twice
        self.assertEqual(classification.logger.info.call_count, 2)
        
        # Check the specific calls to logger.info
        classification.logger.info.assert_any_call("Training summary saved to /mock/base/path/hedge_export/assets/training_summary.txt")        

    @patch('classification.random_forest.train.src.main.task.shutil.copy2')
    @patch('classification.random_forest.train.src.main.task.os.makedirs')
    @patch('classification.random_forest.train.src.main.task.Classification.create_and_save_summary')
    @patch('classification.random_forest.train.src.main.task.Classification.get_env_vars')
    @patch('classification.random_forest.train.src.main.task.Classification.train')
    @patch('classification.random_forest.train.src.main.task.Classification.save_artifacts')
    def test_execute_training_pipeline(self, mock_save_artifacts, mock_train, mock_get_env_vars,
                                       mock_create_and_save_summary, mock_os_makedirs, mock_copy2):
        classification = Classification()
        classification.data_util = MagicMock()
        classification.data_util.base_path = '/mock/base/path'
        classification.model_dir = '/mock/job/dir'
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS)  

        classification.execute_training_pipeline()

        mock_get_env_vars.assert_called_once()
        mock_train.assert_called_once()
        mock_create_and_save_summary.assert_called_once()
        mock_save_artifacts.assert_called_once()

    @patch('classification.random_forest.train.src.main.task.Classification.create_and_save_summary')
    @patch('classification.random_forest.train.src.main.task.Classification.get_env_vars')
    @patch('classification.random_forest.train.src.main.task.Classification.train')
    @patch('classification.random_forest.train.src.main.task.Classification.save_artifacts')
    @patch('classification.random_forest.train.src.main.task.LoggerUtil')
    @patch('classification.random_forest.train.src.main.task.traceback.format_exc')
    def test_execute_training_pipeline_exception(self, mock_format_exc, mock_logger, mock_save_artifacts, mock_train,
                                                 mock_get_env_vars, mock_create_and_save_summary):
        classification = Classification()
        classification.data_util = MagicMock()
        classification.data_util.base_path = '/mock/base/path'
        classification.model_dir = '/mock/job/dir'
        classification.pipeline_status = HedgePipelineStatus(status=Status.FAILURE)

        mock_train.side_effect = Exception("Training error")

        classification.execute_training_pipeline()

        mock_get_env_vars.assert_called_once()
        mock_train.assert_called_once()
        mock_create_and_save_summary.assert_not_called()
        mock_save_artifacts.assert_not_called()
        classification.data_util.update_status.assert_called_once_with(False, 'An unexpected error occurred: Training error\n')
        mock_format_exc.assert_called_once()
    
    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "featureNameToColumnIndex": {}
    })
    @patch('classification.random_forest.train.src.main.task.pd.read_csv')
    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    def test_output_feature_all_nan(self, mock_feature_extractor, mock_read_csv, mock_load_json):
        classification = Classification()
        classification.logger = MagicMock()
        classification.data_util = MagicMock()
        classification.data_util.base_path = '/mock/base/path'
        
        classification.pipeline_status = HedgePipelineStatus()
        
        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor_instance.get_data_object.return_value = {"featureNameToColumnIndex": {}}
        mock_feature_extractor.return_value = mock_feature_extractor_instance

        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_read_csv.return_value = MagicMock()
    
        # Mock the FeatureExtractor
        mock_feature_extractor = MagicMock()
        mock_feature_extractor.get_input_features_list.return_value = ['input_feature1', 'input_feature2']
        mock_feature_extractor.get_output_features_list.return_value = ['output_feature']
    
        with patch('classification.random_forest.train.src.main.task.FeatureExtractor', return_value=mock_feature_extractor):
            with self.assertRaises(HedgeTrainingException) as context:
                classification.train()    
            self.assertIn("Output feature data is empty", (str(context.exception)))
    
    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "featureNameToColumnIndex": {}
    })
    @patch('classification.random_forest.train.src.main.task.pd.read_csv')
    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    def test_empty_output_feature(self, mock_feature_extractor, mock_read_csv, mock_load_json):
        classification = Classification()
        classification.logger = MagicMock()
        classification.data_util = MagicMock()
        classification.data_util.base_path = '/mock/base/path'
        classification.pipeline_status = HedgePipelineStatus()
        
        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor_instance.get_data_object.return_value = {"featureNameToColumnIndex": {}}
        
        mock_feature_extractor.return_value = mock_feature_extractor_instance

        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_df = pd.DataFrame({'input_feature': [1, 2, 3], 'output_feature': [np.nan, np.nan, np.nan]})
        mock_read_csv.return_value = mock_df
        
        # Mock the FeatureExtractor
        mock_feature_extractor = MagicMock()
        mock_feature_extractor.get_input_features_list.return_value = ['input_feature']
        mock_feature_extractor.get_output_features_list.return_value = ['output_feature']
    
        with patch('classification.random_forest.train.src.main.task.FeatureExtractor', return_value=mock_feature_extractor):
            with self.assertRaises(HedgeTrainingException) as context:
                classification.train()    
            self.assertIn("Output feature data is empty", (str(context.exception)))
    
    
    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "featureNameToColumnIndex": {}
    })
    @patch('classification.random_forest.train.src.main.task.pd.read_csv')
    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    def test_numerical_output_feature(self, mock_feature_extractor, mock_read_csv, mock_load_json):
        classification = Classification()
        classification.logger = MagicMock()
        classification.data_util = MagicMock()
        classification.data_util.base_path = '/mock/base/path'
        classification.pipeline_status = HedgePipelineStatus()
        
        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor_instance.get_data_object.return_value = {"featureNameToColumnIndex": {}}
        
        mock_feature_extractor.return_value = mock_feature_extractor_instance

        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_df = pd.DataFrame({'input_feature': [1, 2, 3], 'output_feature': [10, 20, 30]})
        mock_read_csv.return_value = mock_df
        
        # Mock the FeatureExtractor
        mock_feature_extractor = MagicMock()
        mock_feature_extractor.get_input_features_list.return_value = ['input_feature']
        mock_feature_extractor.get_output_features_list.return_value = ['output_feature']
    
        with patch('classification.random_forest.train.src.main.task.FeatureExtractor', return_value=mock_feature_extractor):
            with self.assertRaises(HedgeTrainingException) as context:
                classification.train()    
            self.assertIn("Output feature data cannot have non-string or non-object data", (str(context.exception)))


    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "featureNameToColumnIndex": {}
    })
    @patch('classification.random_forest.train.src.main.task.pd.read_csv')
    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    @patch('classification.random_forest.train.src.main.task.RandomForestClassifier')
    @patch('classification.random_forest.train.src.main.task.joblib.dump')
    def test_categorical_output_feature(self, mock_joblib_dump, mock_rf_classifier,
                                                      mock_feature_extractor, mock_read_csv, mock_load_json):
        classification = Classification()
        classification.logger = MagicMock()
        classification.data_util = MagicMock()
        classification.data_util.base_path = '/mock/base/path'
        classification.pipeline_status = HedgePipelineStatus()
        
        mock_feature_extractor_instance = MagicMock()
        mock_feature_extractor_instance.get_data_object.return_value = {"featureNameToColumnIndex": {}}
        
        mock_feature_extractor.return_value = mock_feature_extractor_instance

        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_df = pd.DataFrame({'input_feature': [1, 2, 3, 4, 5], 'output_feature': ["10", "20", "10", "20", "10"]})
        mock_read_csv.return_value = mock_df
        
        # Mock the FeatureExtractor
        mock_feature_extractor = MagicMock()
        mock_feature_extractor.get_input_features_list.return_value = ['input_feature']
        mock_feature_extractor.get_output_features_list.return_value = ['output_feature']
        
        classification.classification_report = MagicMock()
        
        # Mock the RandomForestClassifier
        mock_rf_instance = MagicMock()
        mock_rf_instance.predict.return_value = np.array([1])
        mock_rf_classifier.return_value = mock_rf_instance
    
        with patch('classification.random_forest.train.src.main.task.FeatureExtractor', return_value=mock_feature_extractor):
            classification.train()    
        
        # Assert that the model was trained and predicted
        mock_rf_instance.fit.assert_called_once()
        mock_rf_instance.predict.assert_called_once()



    @patch('classification.random_forest.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars_key_error(self, mock_get_env_value):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()

        mock_get_env_value.side_effect = KeyError("NUM_TREES not found")
        with self.assertRaises(HedgeTrainingException) as context:
            classification.get_env_vars()
        self.assertIn("Environment variable is missing", str(context.exception))
        self.assertEqual(context.exception.error_code, 1001)

    @patch('classification.random_forest.train.src.main.task.EnvironmentUtil.get_env_value')
    def test_get_env_vars_unexpected_exception(self, mock_get_env_value):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()

        mock_get_env_value.side_effect = Exception("Unexpected error")
        with self.assertRaises(HedgeTrainingException) as context:
            classification.get_env_vars()
        self.assertIn("Unexpected error in get_env_vars:", str(context.exception))
        self.assertEqual(context.exception.error_code, 1002)

    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    @patch('classification.random_forest.train.src.main.task.pd.read_csv')
    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "mlModelConfig": {
            "mlDataSourceConfig": {
                "featuresByProfile": {
                    "defaultProfile": [
                        {"name": "feature1", "isInput": True, "type": "METRIC"}
                    ]
                }
            }
        }
    })
    def test_train_no_output_feature(self, mock_load_json, mock_read_csv, mock_feature_extractor):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()
        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        df = pd.DataFrame(np.random.rand(10, 1), columns=["feature1"])
        mock_read_csv.return_value = df

        mock_fe_instance = MagicMock()
        mock_fe_instance.get_input_features_list.return_value = ["feature1"]
        mock_fe_instance.get_output_features_list.return_value = []  # no output features
        mock_feature_extractor.return_value = mock_fe_instance

        with self.assertRaises(HedgeTrainingException) as context:
            classification.train()
        self.assertIn("Training failed due to invalid data", str(context.exception))
        self.assertEqual(context.exception.error_code, 2001)

    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    @patch('classification.random_forest.train.src.main.task.pd.read_csv',
           side_effect=Exception("Mocked read_csv error"))
    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "mlModelConfig": {"mlDataSourceConfig": {"featuresByProfile": {}}}
    })
    def test_train_unexpected_exception(self, mock_load_json, mock_read_csv, mock_feature_extractor):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()
        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )

        mock_fe_instance = MagicMock()
        mock_fe_instance.get_input_features_list.return_value = ["feature1"]
        mock_fe_instance.get_output_features_list.return_value = ["target"]
        mock_feature_extractor.return_value = mock_fe_instance

        with self.assertRaises(HedgeTrainingException) as context:
            classification.train()
        self.assertIn("Unexpected error in train:", str(context.exception))
        self.assertEqual(context.exception.error_code, 2002)

    @patch('classification.random_forest.train.src.main.task.os.makedirs', side_effect=IOError("Mocked IOError"))
    def test_save_artifacts_IO_error(self, mock_makedirs):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()
        classification.model_dir = "/mock/job/dir"
        classification.algo_name = "mock_algo"
        classification.data_util.base_path = "/mock/base/path"

        with self.assertRaises(HedgeTrainingException) as context:
            classification.save_artifacts({}, MagicMock())
        self.assertIn("Failed to save artifacts", str(context.exception))

    @patch('classification.random_forest.train.src.main.task.os.makedirs')
    @patch('classification.random_forest.train.src.main.task.joblib.dump', side_effect=Exception("Mocked dump error"))
    def test_save_artifacts_unexpected_exception(self, mock_joblib_dump, mock_makedirs):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()
        classification.model_dir = "/mock/job/dir"
        classification.algo_name = "mock_algo"
        classification.data_util.base_path = "/mock/base/path"
    
        with self.assertRaises(HedgeTrainingException) as context:
            classification.save_artifacts({"key": "value"}, MagicMock())
        self.assertIn("Unexpected error in save_artifacts:", str(context.exception))
        self.assertEqual(context.exception.error_code, 4002)
    
    
    @patch('classification.random_forest.train.src.main.task.os.makedirs', side_effect=RuntimeError("Mocked RuntimeError"))
    def test_create_and_save_summary_runtime_error(self, mock_makedirs):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()
        classification.data_util.base_path = "/mock/base/path"
    
        with self.assertRaises(HedgeTrainingException) as context:
            classification.create_and_save_summary()
        self.assertIn("Failed to generate summary", str(context.exception))
        self.assertEqual(context.exception.error_code, 3001)
    
    
    @patch('classification.random_forest.train.src.main.task.os.makedirs')
    @patch('classification.random_forest.train.src.main.task.shutil.copy2', side_effect=Exception("Mocked generic exception"))
    def test_create_and_save_summary_unexpected_exception(self, mock_copy2, mock_makedirs):
        classification = Classification()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.data_util = MagicMock()
        classification.data_util.base_path = "/mock/base/path"
    
        with self.assertRaises(HedgeTrainingException) as context:
            classification.create_and_save_summary()
        self.assertIn("Unexpected error in create_summary:", str(context.exception))
        self.assertEqual(context.exception.error_code, 3002)

    @patch('classification.random_forest.train.src.main.task.LoggerUtil')
    @patch('classification.random_forest.train.src.main.task.Classification.get_env_vars',
           side_effect=HedgeTrainingException("Simulated hedge exception", error_code=9999))
    @patch('classification.random_forest.train.src.main.task.traceback.format_exc', return_value="Formatted traceback")
    def test_execute_training_pipeline_hedge_exception(self, mock_format_exc, mock_get_env_vars, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger

        classification = Classification()
        classification.data_util = MagicMock()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.execute_training_pipeline()

        mock_logger.error.assert_any_call("Training pipeline failed with error: [Error 9999] Simulated hedge exception")

    @patch('classification.random_forest.train.src.main.task.LoggerUtil')
    @patch('classification.random_forest.train.src.main.task.Classification.get_env_vars')
    @patch('classification.random_forest.train.src.main.task.Classification.train',
           side_effect=Exception("Unexpected error in pipeline"))
    @patch('classification.random_forest.train.src.main.task.traceback.format_exc', return_value="Formatted traceback")
    def test_execute_training_pipeline_unexpected_exception(self, mock_format_exc, mock_train, mock_get_env_vars,
                                                            mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger

        classification = Classification()
        classification.data_util = MagicMock()
        classification.pipeline_status = HedgePipelineStatus(status=Status.SUCCESS, message="")
        classification.execute_training_pipeline()

        mock_logger.error.assert_any_call("An unexpected error occurred: Unexpected error in pipeline")

    @patch('classification.random_forest.train.src.main.task.os.makedirs')
    @patch('classification.random_forest.train.src.main.task.joblib.dump')
    @patch('classification.random_forest.train.src.main.task.os.chdir')
    @patch('classification.random_forest.train.src.main.task.shutil.make_archive',
           return_value='/mock/job/dir/hedge_export.zip')
    def test_save_artifacts_successful(self, mock_make_archive, mock_os_chdir, mock_joblib_dump, mock_os_makedirs):
        classification = Classification()
        classification.logger = MagicMock()  # Mock logger so we can verify info calls
        classification.data_util = MagicMock()
        classification.data_util.base_path = "/mock/base/path"
        classification.model_dir = "/mock/job/dir"
        classification.algo_name = "mock_algo"

        artifact_dictionary = {'key': 'value'}
        model = MagicMock()

        classification.save_artifacts(artifact_dictionary, model)

        mock_os_makedirs.assert_called_once_with('/mock/base/path/hedge_export', exist_ok=True)
        mock_joblib_dump.assert_any_call(artifact_dictionary, '/mock/base/path/hedge_export/artifacts.gz',
                                         compress=True)
        mock_joblib_dump.assert_any_call(model, '/mock/base/path/hedge_export/model.gz', compress=True)
        classification.logger.info.assert_any_call(
            'Artifact dictionary file created at: /mock/base/path/hedge_export/artifacts.gz')
        classification.logger.info.assert_any_call('Model file created at: /mock/base/path/hedge_export/model.gz')

        mock_os_chdir.assert_called_once_with('/mock/job/dir')
        mock_make_archive.assert_called_once_with('hedge_export', 'zip', root_dir=".", base_dir='./mock_algo')
        classification.data_util.upload_data.assert_called_once_with('/mock/job/dir/hedge_export.zip')

    @patch('classification.random_forest.train.src.main.task.Classification.get_env_vars')
    @patch('classification.random_forest.train.src.main.task.Classification.train')
    @patch('classification.random_forest.train.src.main.task.Classification.create_and_save_summary')
    @patch('classification.random_forest.train.src.main.task.Classification.save_artifacts')
    def test_execute_training_pipeline_success(self, mock_save_artifacts, mock_create_summary, mock_train, mock_get_env_vars):
        classification = Classification()
        classification.data_util = MagicMock()
        classification.pipeline_status = HedgePipelineStatus(message="")
        classification.execute_training_pipeline()
    
        self.assertTrue(classification.pipeline_status.is_success)
        self.assertEqual(classification.pipeline_status.message, "End of Training\n")
        classification.data_util.update_status.assert_called_once_with(True, "End of Training\n")

    @patch('classification.random_forest.train.src.main.task.os.path.exists', return_value=True)
    @patch('classification.random_forest.train.src.main.task.Classification.load_json', return_value={
        "mlModelConfig": {
            "mlDataSourceConfig": {
                "featuresByProfile": {
                    "defaultProfile": [
                        {"name": "feature1", "isInput": True, "type": "METRIC"},
                        {"name": "feature2", "isInput": True, "type": "METRIC"},
                        {"name": "target", "isOutput": True, "type": "METRIC"}
                    ]
                }
            }
        }
    })
    @patch('classification.random_forest.train.src.main.task.pd.read_csv')
    @patch('classification.random_forest.train.src.main.task.FeatureExtractor')
    @patch('classification.random_forest.train.src.main.task.DataFramePreprocessor')
    def test_train_pipeline_coverage(
            self, mock_dataframe_preprocessor, mock_feature_extractor, mock_read_csv, mock_load_json, mock_path_exists
    ):
        classification = Classification()
        classification.logger = MagicMock()
        classification.data_util = MagicMock()
        classification.data_util.read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/config.json",
            csv_file_path="/mock/data.csv"
        )
        classification.pipeline_status = HedgePipelineStatus()
        
        # Use string target values
        df = pd.DataFrame({
            'feature1': [1.0, 2.0, None, 4.0],
            'feature2': [10.0, None, 30.0, 40.0],
            'target': ["classA", "classB", "classB", "classA"]
        })
        mock_read_csv.return_value = df

        mock_fe_instance = MagicMock()
        mock_fe_instance.get_input_features_list.return_value = ['feature1', 'feature2']
        mock_fe_instance.get_output_features_list.return_value = ['target']
        mock_feature_extractor.return_value = mock_fe_instance

        mock_preprocessor_instance = MagicMock()
        mock_preprocessor_instance.fit_transform.side_effect = lambda x: x.values
        mock_preprocessor_instance.transform.side_effect = lambda x: x.values
        mock_dataframe_preprocessor.return_value = mock_preprocessor_instance

        classification.train()

        classification.logger.info.assert_any_call('Shape of original X: (4, 2)')
        classification.logger.info.assert_any_call('Shape of original X_train: (3, 2)')
        classification.logger.info.assert_any_call('Shape of transformed X_train: (3, 2)')
        self.assertIn('input_transformer', classification.artifact_dictionary)
        self.assertIn('label_encoder', classification.artifact_dictionary)
        self.assertIsNotNone(classification.model)

    def test_repr(self):
        classification = Classification()
        classification.num_trees = 200
        classification.max_depth = 10
        classification.min_samples_leaf = 2
        classification.min_samples_split = 4

        rep_str = repr(classification)
        self.assertIn("num_trees: 200", rep_str)
        self.assertIn("max_depth: 10", rep_str)
        self.assertIn("min_samples_leaf: 2", rep_str)
        self.assertIn("min_samples_split: 4", rep_str)


if __name__ == '__main__':
    unittest.main()
