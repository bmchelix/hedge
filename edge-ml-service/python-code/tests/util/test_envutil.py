"""
Test for EnvironmentUtil
"""

import sys
from typing import Dict, Any
import unittest
from unittest.mock import patch
from common.src.util.env_util import EnvironmentUtil

class TestEnvironmentUtil(unittest.TestCase):
    """
    Test Class for EnvironmentUtil
    """
    @patch('common.src.util.env_util.create_dict_from_yaml')
    def setUp(self, mock_create_dict):
        mock_create_dict.return_value = {
                'ModelDir': '/res/edge/models',
               'training_file_id': '/tmp/HedgeAnomaly/HedgeAnomalyTrainingData1/training_input.zip',
                'JobDir': '/tmp/hedge',
                'Local': False,
                'Service.host': '0.0.0.0' ,
                'Service.port': '48096'
            }
        self.env_util = EnvironmentUtil('new.yaml')

    def test_env_util(self):
        """
        test for EnvironmentUtil 
        """
        config: Dict[str, Any] = self.env_util.config
        assert config.get('ModelDir') == '/res/edge/models'
        assert config.get('training_file_id') == '/tmp/HedgeAnomaly/HedgeAnomalyTrainingData1/training_input.zip'
        assert config.get('JobDir') == '/tmp/hedge'
        assert config.get('Service.host') == '0.0.0.0'
        assert config.get('Service.port') == '48096'
        assert not config.get('Local')


    def test_parse_command_line_args(self):
        """
        Test arguments passed to EnvironmentUtil
        """
        old_sys_argv = sys.argv
        sys.argv = [old_sys_argv[0]] + ['--a','test', "--b", "1"]
        env_util = EnvironmentUtil(args=["a", "b"])
        assert env_util.config.get("a") == "test"
        assert env_util.config.get("b") == "1"
        sys.argv = old_sys_argv
