import unittest
from unittest.mock import patch
from common.src.ml.hedge_status import HedgePipelineStatus, Status

class TestHedgePipelineStatus(unittest.TestCase):

    def test_default_initialization(self):
        status = HedgePipelineStatus()
        self.assertEqual(status.status, Status.STARTED)
        self.assertEqual(status.message, "")
        self.assertFalse(status.is_success)

    @patch('os.environ.get')
    def test_update_status(self, mock_env_get):
        mock_env_get.side_effect = {
            'LOCAL': 'true',
            'MODELDIR': '/mock/job/dir',
            'JOB_DIR': '/mock/job/dir',
            'TRAINING_FILE_ID': 'mock_training_file_id',
        }.get
        status = HedgePipelineStatus(file_path="status.json")
        self.assertEqual(status.status, Status.STARTED)
        
        status.update(new_status=Status.FAILURE)
        self.assertEqual(status.status, Status.FAILURE)
        self.assertFalse(status.is_success)
        
    def test_update_message_append(self):
        status = HedgePipelineStatus(file_path="status.json")
        status.update(new_message="First message")
        status.update(new_message="Second message")
        
        self.assertEqual(status.message, "First message\nSecond message\n")
        self.assertEqual(status.status, Status.STARTED)  # Ensure status remains unchanged
        

    def test_is_success_property(self):
        status = HedgePipelineStatus(file_path="status.json")
        self.assertTrue(status.status == Status.STARTED)
        
        status.update(new_status=Status.FAILURE)
        self.assertFalse(status.is_success)
        
        status.update(new_status=Status.SUCCESS)
        self.assertTrue(status.is_success)
        

    def test_is_success_when_failure(self):
        status = HedgePipelineStatus(file_path="status.json")
        status.update(new_status=Status.FAILURE)
        self.assertFalse(status.is_success)
        


    def test_string_representation(self):
        status = HedgePipelineStatus(file_path="status.json")
        status.update(new_status=Status.FAILURE, new_message="Test message")
        
        expected_str = "Status: FAILURE, Message: Test message\n"
        self.assertEqual(str(status), expected_str)
        

    def test_multiple_updates(self):
        status = HedgePipelineStatus(file_path="status.json")
        
        status.update(new_status=Status.FAILURE, new_message="First failure")
        self.assertEqual(status.status, Status.FAILURE)
        self.assertEqual(status.message, "First failure\n")
        self.assertFalse(status.is_success)
        
        status.update(new_message="Additional info")
        self.assertEqual(status.status, Status.FAILURE)
        self.assertEqual(status.message, "First failure\nAdditional info\n")
        self.assertFalse(status.is_success)
        
        status.update(new_status=Status.SUCCESS, new_message="Fixed")
        self.assertEqual(status.status, Status.SUCCESS)
        self.assertEqual(status.message, "First failure\nAdditional info\nFixed\n")
        self.assertTrue(status.is_success)
        
        expected_str = "Status: SUCCESS, Message: First failure\nAdditional info\nFixed\n"
        self.assertEqual(str(status), expected_str)

    def test_preserve_message_when_updating_status(self):
        status = HedgePipelineStatus(file_path="status.json")
        status.update(new_message="Initial message")
        initial_message = status.message
        
        status.update(new_status=Status.FAILURE)
        
        self.assertEqual(status.status, Status.FAILURE)
        self.assertEqual(status.message, initial_message)
        self.assertFalse(status.is_success)                            

if __name__ == '__main__':
    unittest.main()