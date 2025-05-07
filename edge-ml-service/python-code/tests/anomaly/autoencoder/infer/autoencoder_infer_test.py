"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import unittest
from unittest.mock import patch, MagicMock
import os
import sys
import numpy as np
from anomaly.autoencoder.infer.src.main.task import AutoEncoderInference, AutoEncoderInputs



project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../.."))
if project_root not in sys.path:
    sys.path.insert(0, project_root)

class TestAutoEncoderInference(unittest.TestCase):

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    @patch('anomaly.autoencoder.infer.src.main.task.EnvironmentUtil.get_env_value')
    def test_init(self, mock_env_value, mock_logger_util):
        mock_env_value.side_effect = {
            'LOCAL': 'true',
            'MODELDIR': '/mock/output',
            'PORT': '8000',
            'HOST': '127.0.0.1'
        }.get

        autoencoder_inference = AutoEncoderInference()

        mock_logger_util.assert_called_once()
        self.assertIsNotNone(autoencoder_inference.logger)

        self.assertEqual(autoencoder_inference.local, 'true')
        self.assertEqual(autoencoder_inference.model_dir, '/mock/output')
        self.assertEqual(autoencoder_inference.port, '8000')
        self.assertEqual(autoencoder_inference.host, '127.0.0.1')

    @patch('anomaly.autoencoder.infer.src.main.task.FeatureExtractor')
    @patch('pandas.DataFrame')
    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict(self, mock_logger, mock_dataframe, mock_feature_extractor):
        autoencoder_inference = AutoEncoderInference()
        mock_model = MagicMock()
        mock_transformer = MagicMock()

        mock_model.predict.return_value = np.random.rand(2, 2)
        mock_transformer.transform.return_value = np.random.rand(2, 2)

        autoencoder_inference.model_dict = {
            "models": {"autoencoder": {"config_1": mock_model}},
            "artifacts": {"autoencoder": {
                "config_1": {"scaler": None, "mse_median": 0.1, "median_absolute_deviation": 0.05,
                             "transformer": mock_transformer}}},
            "config": {"autoencoder": {"config_1": {}}}
        }

        mock_feature_extractor.return_value.get_data_object.return_value = {
            "featureNameToColumnIndex": {"feature1": 0, "feature2": 1}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        mock_model.predict.assert_called_once()
        mock_transformer.transform.assert_called_once()


    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict_models_not_loaded(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()
        autoencoder_inference.logger = MagicMock()

        autoencoder_inference.model_dict = {
            "models": {},
            "artifacts": {},
            "config": {}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        self.assertIsNone(result)

        autoencoder_inference.logger.error.assert_any_call("Please load some models")
        autoencoder_inference.logger.error.assert_any_call(
            "Prediction failed: Models have not been loaded. Please initialize the models."
        )

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict_no_ml_algorithm(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()
        autoencoder_inference.logger = MagicMock()

        autoencoder_inference.model_dict = {
            "models": {"different_algorithm": {}},
            "artifacts": {},
            "config": {}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        self.assertIsNone(result)

        autoencoder_inference.logger.error.assert_any_call(
            "Prediction failed: No ML Algorithm called autoencoder"
        )

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_postprocess_predictions(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()

        inputs = np.array([[1.0, 2.0], [2.0, 3.0]])
        outputs = np.array([[1.1, 2.1], [1.9, 2.9]])

        mse_median = 0.1
        mad = 0.05

        result = autoencoder_inference._AutoEncoderInference__postprocess_predictions(inputs, outputs, mse_median, mad)

        self.assertEqual(len(result), len(inputs))
        self.assertTrue(all(isinstance(x, float) for x in result))

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_modified_z_score_scoring(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()

        mse_value = 0.2
        mse_median = 0.1
        mad = 0.05

        result = autoencoder_inference._AutoEncoderInference__modified_z_score_scoring(mse_value, mse_median, mad)

        self.assertIsInstance(result, float)
        self.assertAlmostEqual(result, 1.349)

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict_training_config_not_found(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()
        autoencoder_inference.logger = MagicMock()

        autoencoder_inference.model_dict = {
            "models": {"autoencoder": {"other_config": MagicMock()}},
            "artifacts": {"autoencoder": {"other_config": MagicMock()}},
            "config": {"autoencoder": {"other_config": {}}}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        self.assertIsNone(result)

        autoencoder_inference.logger.error.assert_any_call("No model called config_1")
        autoencoder_inference.logger.error.assert_any_call("Prediction failed: The model config_1 does not exist")

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict_training_artifact_not_found(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()
        autoencoder_inference.logger = MagicMock()

        autoencoder_inference.model_dict = {
            "models": {"autoencoder": {"config_1": MagicMock()}},
            "artifacts": {"autoencoder": {}},
            "config": {"autoencoder": {"config_1": {}}}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        self.assertIsNone(result)

        autoencoder_inference.logger.error.assert_any_call("No training artifact called None")
        autoencoder_inference.logger.error.assert_any_call(
            "Prediction failed: The training artifact None does not exist")

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict_config_not_found(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()
        autoencoder_inference.logger = MagicMock()

        autoencoder_inference.model_dict = {
            "models": {"autoencoder": {"config_1": MagicMock()}},
            "artifacts": {"autoencoder": {"config_1": {"transformer": MagicMock()}}},
            "config": {"autoencoder": {}}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        self.assertIsNone(result)

        autoencoder_inference.logger.error.assert_called_with(
            "Prediction failed: argument of type 'NoneType' is not iterable"
        )

    @patch('anomaly.autoencoder.infer.src.main.task.FeatureExtractor')
    @patch('pandas.DataFrame')
    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_predict_exception_in_prediction(self, mock_logger, mock_dataframe, mock_feature_extractor):
        autoencoder_inference = AutoEncoderInference()
        autoencoder_inference.logger = MagicMock()
        mock_model = MagicMock()
        mock_transformer = MagicMock()

        mock_model.predict.side_effect = Exception("Mocked exception during model prediction")

        autoencoder_inference.model_dict = {
            "models": {"autoencoder": {"config_1": mock_model}},
            "artifacts": {"autoencoder": {
                "config_1": {"transformer": mock_transformer, "mse_median": 0.1, "median_absolute_deviation": 0.05}}},
            "config": {"autoencoder": {"config_1": {}}}
        }

        mock_feature_extractor.return_value.get_data_object.return_value = {
            "featureNameToColumnIndex": {"feature1": 0, "feature2": 1}
        }

        inputs = AutoEncoderInputs(__root__={
            "id1": [0.5, 1.0],
            "id2": [1.5, 2.0]
        })

        result = autoencoder_inference.predict("autoencoder", "config_1", inputs)

        self.assertIsNone(result)

        autoencoder_inference.logger.error.assert_called_with(
            "Prediction failed: Mocked exception during model prediction")

    @patch('anomaly.autoencoder.infer.src.main.task.LoggerUtil')
    def test_modified_z_score_scoring_mad_zero(self, mock_logger):
        autoencoder_inference = AutoEncoderInference()

        mse_value = 0.2
        mse_median = 0.1
        mad = 0.0

        result = autoencoder_inference._AutoEncoderInference__modified_z_score_scoring(mse_value, mse_median, mad)

        self.assertEqual(result, 0)
        autoencoder_inference.logger.warning.assert_called_with(
            "Median Absolute Deviation is zero. Returning zero as Z-score.")


if __name__ == '__main__':
    unittest.main()
