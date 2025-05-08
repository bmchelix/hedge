"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import unittest
from unittest.mock import patch, MagicMock
import numpy as np

from classification.random_forest.infer.src.main.task import RandomForestInference, RandomForestInputs


class TestRandomForestInference(unittest.TestCase):

    @patch('classification.random_forest.infer.src.main.task.LoggerUtil')
    @patch('classification.random_forest.infer.src.main.task.EnvironmentUtil.get_env_value')
    def test_init(self, mock_env_value, mock_logger_util):
        mock_env_value.side_effect = {
            'LOCAL': True,
            'MODELDIR': '/mock/output',
            'PORT': '8000',
            'HOST': '127.0.0.1'
        }.get

        rf_inference = RandomForestInference()

        mock_logger_util.assert_called_once()
        self.assertIsNotNone(rf_inference.logger)

        self.assertTrue(rf_inference.local)
        self.assertEqual(rf_inference.model_dir, '/mock/output')
        self.assertEqual(rf_inference.port, '8000')
        self.assertEqual(rf_inference.host, '127.0.0.1')

    @patch('classification.random_forest.infer.src.main.task.FeatureExtractor')
    @patch('pandas.DataFrame')
    @patch('classification.random_forest.infer.src.main.task.LoggerUtil')
    def test_predict(self, mock_logger, mock_dataframe, mock_feature_extractor):
        rf_inference = RandomForestInference()
        mock_model = MagicMock()
        mock_transformer = MagicMock()
        mock_label_encoder = MagicMock()

        mock_model.predict.return_value = np.array([1, 0])
        mock_model.predict_proba.return_value = np.array([[0.2, 0.8], [0.6, 0.4]])
        mock_label_encoder.inverse_transform.return_value = ['class1', 'class2']
        mock_transformer.transform.return_value = np.random.rand(2, 2)

        rf_inference.model_dict = {
            "models": {"random_forest": {"config_1": mock_model}},
            "artifacts": {
                "random_forest": {
                    "config_1": {
                        "input_transformer": mock_transformer,
                        "label_encoder": mock_label_encoder
                    }
                }
            },
            "config": {"random_forest": {"config_1": {}}}
        }

        mock_feature_extractor.return_value.get_input_features_list.return_value = ["feature1", "feature2"]

        inputs = RandomForestInputs(__root__={
            'id1': [0.5, 1.0],
            'id2': [1.5, 2.0]
        })

        result = rf_inference.predict("random_forest", "config_1", inputs)

        mock_model.predict.assert_called_once()
        mock_transformer.transform.assert_called_once()
        mock_label_encoder.inverse_transform.assert_called_once()

        # Use by_alias=True to get the serialized keys
        result_dict = result.dict(by_alias=True)['__root__']

        expected_output = {
            'id1': {'class': 'class1', 'confidence': 0.8},
            'id2': {'class': 'class2', 'confidence': 0.6}
        }

        self.assertEqual(result_dict, expected_output)

    @patch('classification.random_forest.infer.src.main.task.LoggerUtil')
    def test_clear_and_initialize_model_dict(self, mock_logger):
        rf_inference = RandomForestInference()
        rf_inference.model_dict = {
            "models": {"random_forest": {"config_1": "mock_model"}},
            "config": {"random_forest": {"config_1": {}}},
            "input_transformer": {"random_forest": {"config_1": "mock_transformer"}},
            "label_encoder": {"random_forest": {"config_1": "mock_label_encoder"}}
        }

        keys_to_clear = ["models", "config", "input_transformer", "label_encoder"]

        returned_dict = rf_inference.clear_and_initialize_model_dict(keys_to_clear)

        expected_dict = {key: {} for key in keys_to_clear}

        self.assertEqual(returned_dict, expected_dict)

        self.assertNotEqual(rf_inference.model_dict, expected_dict)
        self.assertIn("random_forest", rf_inference.model_dict["models"])

    @patch('classification.random_forest.infer.src.main.task.LoggerUtil')
    def test_predict_models_not_loaded(self, mock_logger):
        rf_inference = RandomForestInference()

        rf_inference.model_dict = {
            "models": {},
            "artifacts": {},
            "config": {}
        }

        inputs = RandomForestInputs(__root__={
            'id1': [0.5, 1.0],
            'id2': [1.5, 2.0]
        })

        with self.assertRaises(RuntimeError) as context:
            rf_inference.predict("random_forest", "config_1", inputs)

        self.assertIn("Models have not been loaded", str(context.exception))

    @patch('classification.random_forest.infer.src.main.task.LoggerUtil')
    def test_predict_no_ml_algorithm(self, mock_logger_util):
        # Set up the mock logger
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger

        rf_inference = RandomForestInference()

        # Set up model_dict with 'models' not empty but missing 'random_forest' key
        rf_inference.model_dict = {
            "models": {
                "some_other_algorithm": {"config_1": MagicMock()}
            },
            "artifacts": {
                "some_other_algorithm": {"config_1": {
                    "input_transformer": MagicMock(),
                    "label_encoder": MagicMock()
                }}
            },
            "config": {
                "some_other_algorithm": {"config_1": MagicMock()}
            }
        }

        # Prepare inputs
        inputs = RandomForestInputs(__root__={
            'id1': [0.5, 1.0],
            'id2': [1.5, 2.0]
        })

        # Call predict and expect RuntimeError
        with self.assertRaises(RuntimeError) as context:
            rf_inference.predict("random_forest", "config_1", inputs)

        # Check that the exception message is correct
        self.assertIn("No ML Algorithm called random_forest", str(context.exception))

        # Verify that logger.error was called with the correct message
        mock_logger.error.assert_called_with("Prediction failed: No ML Algorithm called random_forest")


if __name__ == '__main__':
    unittest.main()
