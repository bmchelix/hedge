from unittest.mock import MagicMock, mock_open, patch
import unittest
from common.src.ml.hedge_inference import HedgeInferenceBase, Inputs
from common.src.util.logger_util import LoggerUtil


# Add this concrete subclass
class ConcreteHedgeInferenceUtil(HedgeInferenceBase):
    def __init__(self):
        self.logger = LoggerUtil().logger

    def read_model_config(self):
        pass

    def predict(self, ml_algorithm: str, training_config: str, external_input: Inputs):
        pass
    
class TestHedgeInferenceUtil(unittest.TestCase):

    @patch('common.src.ml.hedge_inference.os')
    @patch('tests.common.ml.hedge_inference_test.LoggerUtil')
    def test_load_model_initializes_model_dict(self, mock_logger_util, mock_os):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
        mock_os.listdir.return_value = ['algorithm1', 'algorithm2']
        mock_os.path.isfile.return_value = False
        mock_os.path.exists.return_value = True

        # Use the concrete subclass here
        hedge_inference = ConcreteHedgeInferenceUtil()
        artifacts_dict = {'model': 'model.pkl', 'encoder': 'encoder.pkl'}
        base_path = '/test/path'
    
        with patch.object(hedge_inference, '_get_model_cfgs', return_value=(['config1', 'config2'], True)):
            with patch.object(hedge_inference, '_load_model_artifacts', return_value=True):
                result = hedge_inference.load_model(base_path, artifacts_dict)
    
        self.assertEqual(list(result.keys()), list(artifacts_dict.keys()))
        for artifact_type in artifacts_dict:
            self.assertIn('algorithm1', result[artifact_type])
            self.assertIn('algorithm2', result[artifact_type])
            # for algorithm in result[artifact_type]:
            #     self.assertIn('config1', result[artifact_type][algorithm])
            #     self.assertIn('config2', result[artifact_type][algorithm])
    
        #mock_logger.info.assert_called_with("Initialization In Process")
    
    @patch('common.src.ml.hedge_inference.importlib.import_module')
    @patch('common.src.ml.hedge_inference.os.path.exists')
    @patch('common.src.ml.hedge_inference.open', new_callable=mock_open)
    def test_load_artifact_various_file_types(self, mock_open, mock_exists, mock_import_module):
        mock_exists.return_value = True
        mock_logger = MagicMock()

        # Use ConcreteHedgeInferenceUtil instead of self.hedge_inference
        hedge_inference = ConcreteHedgeInferenceUtil()
        hedge_inference.logger = mock_logger
    
        # Test .h5 file
        mock_tf_keras = MagicMock()
        mock_import_module.return_value = mock_tf_keras
        result = hedge_inference._HedgeInferenceBase__load_artifact('model.h5')
        mock_tf_keras.load_model.assert_called_once()
        self.assertEqual(result, mock_tf_keras.load_model.return_value)
    
        # Test .json file
        mock_json = MagicMock()
        mock_import_module.return_value = mock_json
        result = hedge_inference._HedgeInferenceBase__load_artifact('config.json')
        mock_json.load.assert_called_once()
        self.assertEqual(result, mock_json.load.return_value)
    
        # Test joblib file
        mock_joblib = MagicMock()
        mock_import_module.return_value = mock_joblib
        result = hedge_inference._HedgeInferenceBase__load_artifact('model.joblib')
        mock_joblib.load.assert_called_once()
        self.assertEqual(result, mock_joblib.load.return_value)
    

if __name__ == '__main__':
    unittest.main()
