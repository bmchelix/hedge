"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import unittest
from unittest.mock import MagicMock, patch
from common.src.data.impl.local_training_util import LocalTrainingUtil

class TestLocalTrainingUtil(unittest.TestCase):

    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_successful(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        result = local_training_util.update_status(True, "Test message")
    
        self.assertEqual(result, "SUCCESSFUL")
        mock_logger.warning.assert_called_once_with("Reporting status: SUCCESSFUL")
        mock_logger.error.assert_not_called()
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_failure(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        result = local_training_util.update_status(False, "Test error message")
    
        self.assertEqual(result, "FAILED")
        mock_logger.warning.assert_called_once_with("Reporting status: FAILED")
        mock_logger.error.assert_called_once_with("Test error message")
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_logs_warning(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        status = local_training_util.update_status(True, "Test message")
    
        self.assertEqual(status, "SUCCESSFUL")
        mock_logger.warning.assert_called_once_with("Reporting status: SUCCESSFUL")
        mock_logger.error.assert_not_called()
        
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_logs_error_on_failure(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        error_message = "Test error message"
        result = local_training_util.update_status(False, error_message)
    
        mock_logger.warning.assert_called_once_with("Reporting status: FAILED")
        mock_logger.error.assert_called_once_with(error_message)
        self.assertEqual(result, "FAILED")
        
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_success_no_error_log(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        status = local_training_util.update_status(True, "Success message")
    
        self.assertEqual(status, "SUCCESSFUL")
        mock_logger.warning.assert_called_once_with("Reporting status: SUCCESSFUL")
        mock_logger.error.assert_not_called()
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_empty_message(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        result = local_training_util.update_status(False, "")
    
        self.assertEqual(result, "FAILED")
        mock_logger.warning.assert_called_once_with("Reporting status: FAILED")
        mock_logger.error.assert_called_once_with("")
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status_unicode_message(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
        unicode_message = "Unicode test: こんにちは"
        result = local_training_util.update_status(False, unicode_message)
    
        self.assertEqual(result, "FAILED")
        mock_logger.warning.assert_called_once_with("Reporting status: FAILED")
        mock_logger.error.assert_called_once_with(unicode_message)
    
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_update_status(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
    
        local_training_util = LocalTrainingUtil()
    
        # Test successful case
        result_success = local_training_util.update_status(True, "Success message")
        self.assertEqual(result_success, "SUCCESSFUL")
        mock_logger.warning.assert_called_with("Reporting status: SUCCESSFUL")
        mock_logger.error.assert_not_called()
    
        # Test failure case
        result_failure = local_training_util.update_status(False, "Failure message")
        self.assertEqual(result_failure, "FAILED")
        mock_logger.warning.assert_called_with("Reporting status: FAILED")
        mock_logger.error.assert_called_with("Failure message")
    
        # Verify that the status is returned correctly regardless of the message content
        result_empty = local_training_util.update_status(True, "")
        self.assertEqual(result_empty, "SUCCESSFUL")
    
        result_long = local_training_util.update_status(False, "A" * 1000)
        self.assertEqual(result_long, "FAILED")
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_download_data(self, mock_logger_util):
        local_training_util = LocalTrainingUtil()
        local_training_util.training_file_id = "test_file_id"
        
        result = local_training_util.download_data()
        
        self.assertEqual(result, "test_file_id")
    
    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_download_data_with_none_training_file_id(self, mock_logger_util):
        local_training_util = LocalTrainingUtil()
        local_training_util.training_file_id = None
        
        result = local_training_util.download_data()
        
        self.assertIsNone(result)

    @patch('common.src.data.impl.local_training_util.LoggerUtil')
    def test_set_model_dir(self, mock_logger_util):
        local_training_util = LocalTrainingUtil()
        model_dir = "test_model_dir"
        training_file_id = "test_file_id"
        result = local_training_util.set_model_dir(model_dir, training_file_id)
        self.assertEqual(result, model_dir)
    
    
if __name__ == '__main__':
    unittest.main()
