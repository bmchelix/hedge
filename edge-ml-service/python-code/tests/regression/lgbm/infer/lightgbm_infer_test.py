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

from regression.lightgbm.infer.src.main.task import LightGBMRegressionInference, RegressionInputs, \
    RegressionOutputs


class TestLightGBMRegressionInference(unittest.TestCase):

    @patch('regression.lightgbm.infer.src.main.task.LoggerUtil')
    @patch('regression.lightgbm.infer.src.main.task.EnvironmentUtil.get_env_value')
    def test_init(self, mock_get_env_value, mock_logger_util):
        mock_get_env_value.side_effect = lambda key, default: {
            'PORT': '52000',
            'Service.port': '52000'
        }.get(key, default)

        inference = LightGBMRegressionInference()

        mock_logger_util.assert_called_once()
        self.assertIsNotNone(inference.logger)
        self.assertEqual(inference.port, '52000')

        self.assertIn('models', inference.artifacts_dict)
        self.assertIn('artifacts', inference.artifacts_dict)
        self.assertIn('config', inference.artifacts_dict)

    @patch('regression.lightgbm.infer.src.main.task.FeatureExtractor')
    @patch('regression.lightgbm.infer.src.main.task.pd.DataFrame')
    @patch('regression.lightgbm.infer.src.main.task.LoggerUtil')
    def test_predict_success(self, mock_logger, mock_dataframe_cls, mock_feature_extractor_cls):
        inference = LightGBMRegressionInference()
        inference.logger = MagicMock()

        mock_model = MagicMock()
        mock_transformer = MagicMock()
        mock_scaler = MagicMock()

        mock_model.predict.return_value = np.array([10.0, 20.0])
        mock_transformer.transform.return_value = np.array([[1, 2], [3, 4]])
        mock_scaler.inverse_transform.return_value = np.array([[100.0], [200.0]])

        mock_extractor_instance = MagicMock()
        mock_extractor_instance.get_numerical_inputs_list.return_value = ["feature1", "feature2"]
        mock_extractor_instance.get_numerical_outputs_list.return_value = ["target"]
        mock_feature_extractor_cls.return_value = mock_extractor_instance

        inference.model_dict = {
            "models": {"lightgbm_regression": {"config_1": mock_model}},
            "artifacts": {
                "lightgbm_regression": {"config_1": {"transformer": mock_transformer, "scaler": mock_scaler}}},
            "config": {"lightgbm_regression": {"config_1": {"featuresByProfile": {}}}}
        }

        inputs = RegressionInputs(__root__={
            "corr_id_1": [1.1, 2.2],
            "corr_id_2": [3.3, 4.4]
        })

        result = inference.predict("lightgbm_regression", "config_1", inputs)

        mock_model.predict.assert_called_once()
        mock_transformer.transform.assert_called_once()
        mock_scaler.inverse_transform.assert_called_once()

        self.assertIsInstance(result, RegressionOutputs)
        self.assertIn("corr_id_1", result.__root__)
        self.assertIn("corr_id_2", result.__root__)
        self.assertEqual(result.__root__["corr_id_1"], 100.0)
        self.assertEqual(result.__root__["corr_id_2"], 200.0)

    @patch('regression.lightgbm.infer.src.main.task.LoggerUtil')
    def test_predict_no_models_loaded(self, mock_logger):
        inference = LightGBMRegressionInference()
        inference.logger = MagicMock()

        inference.model_dict = {
            "models": {},
            "artifacts": {},
            "config": {}
        }

        inputs = RegressionInputs(__root__={"corr_id_1": [1.0, 2.0]})

        with self.assertRaises(RuntimeError) as context:
            inference.predict("lightgbm_regression", "config_1", inputs)

        self.assertIn("Models have not been loaded", str(context.exception))
        inference.logger.error.assert_any_call("Please load some models")

    @patch('regression.lightgbm.infer.src.main.task.LoggerUtil')
    def test_predict_no_ml_algorithm(self, mock_logger):
        inference = LightGBMRegressionInference()
        inference.logger = MagicMock()

        inference.model_dict = {
            "models": {"different_algorithm": {}},
            "artifacts": {},
            "config": {}
        }

        inputs = RegressionInputs(__root__={"corr_id_1": [1.0, 2.0]})

        with self.assertRaises(RuntimeError) as context:
            inference.predict("lightgbm_regression", "config_1", inputs)

        self.assertIn("No ML Algorithm called lightgbm_regression", str(context.exception))
        inference.logger.error.assert_any_call("No ML Algorithm called lightgbm_regression")


    @patch('regression.lightgbm.infer.src.main.task.FeatureExtractor')
    @patch('regression.lightgbm.infer.src.main.task.LoggerUtil')
    def test_predict_exception_during_prediction(self, mock_logger, mock_feature_extractor_cls):
        inference = LightGBMRegressionInference()
        inference.logger = MagicMock()

        mock_model = MagicMock()
        mock_model.predict.side_effect = Exception("Mocked exception during prediction")
        mock_transformer = MagicMock()

        mock_extractor_instance = MagicMock()
        mock_extractor_instance.get_input_features_list.return_value = ["feature1", "feature2"]
        mock_extractor_instance.get_numerical_outputs_list.return_value = ["target"]
        mock_feature_extractor_cls.return_value = mock_extractor_instance

        inference.model_dict = {
            "models": {"lightgbm_regression": {"config_1": mock_model}},
            "artifacts": {"lightgbm_regression": {"config_1": {"transformer": mock_transformer}}},
            "config": {"lightgbm_regression": {"config_1": {}}}
        }

        inputs = RegressionInputs(__root__={
            "corr_id_1": [1.0, 2.0]
        })

        with self.assertRaises(RuntimeError) as context:
            inference.predict("lightgbm_regression", "config_1", inputs)

        self.assertIn("Mocked exception during prediction", str(context.exception))
        inference.logger.error.assert_any_call("Prediction failed: Mocked exception during prediction")


if __name__ == '__main__':
    unittest.main()