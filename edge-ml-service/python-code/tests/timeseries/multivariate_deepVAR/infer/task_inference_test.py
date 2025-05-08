import os
import numpy as np
import pandas as pd
import sys
from unittest.mock import MagicMock, mock_open, patch
from common.src.util.infer_exception import HedgeInferenceException
import unittest
from timeseries.multivariate_deepVAR.infer.src.main.task import TimeseriesDeepVARInference, TimeseriesInputs

project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../.."))
if project_root not in sys.path:
    sys.path.insert(0, project_root)

class TestTimeseriesDeepVARInference(unittest.TestCase):
    
    # @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    # @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    # def test_predict_empty_input_data(self, mock_env_util, mock_logger_util):
    #     inference = TimeseriesDeepVARInference()
    #     inference.model_dict = {
    #         "models": {"algorithm": {"config": MagicMock()}},
    #         "params": {"algorithm": {"config": {
    #             "target_columns": ["target1", "target2"],
    #             "group_by_cols": ["device"],
    #             "freq": "1H",
    #             "date_field": "timestamp",
    #             "input_columns": ["timestamp", "device", "target1", "target2"],
    #             "output_columns": ["timestamp", "device", "target1", "target2"]
    #         }}}
    #     }
        
    #     empty_input = TimeseriesInputs(__root__={
    #         "dev_1": {
    #             "data": [],
    #             "confidence_interval": {"min": 0.6, "max": 0.8},
    #         }
    #     })
        
    #     with self.assertRaises(HedgeInferenceException) as context:
    #         inference.predict("algorithm", "config", empty_input)
        
    #     self.assertEqual(context.exception.status_code, 400)
    #     self.assertEqual(str(context.exception.detail), "Correlation IDs list is empty")
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_predict_no_models_loaded(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {"models": {}}
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672617600.0, "dev_1", 2000.0, 10.0]],
                "confidence_interval": {"min": 0.6, "max": 0.8},
            }
        })
        
        with self.assertRaises(HedgeInferenceException) as context:
            inference.predict("algorithm", "config", input_data)
        
        self.assertEqual(context.exception.status_code, 500)
        self.assertEqual(str(context.exception.detail), "Error while loading models: Models have not been loaded. Please initialize the models.")

    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    @patch('gluonts.dataset.common.ListDataset')
    def test_predict_multiple_correlation_ids(self, mock_list_dataset, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        mock_predictor = MagicMock()
        mock_forecast = MagicMock()
        mock_forecast.item_id = "dev_1"
        mock_forecast.start_date.to_timestamp.return_value = pd.Timestamp("2023-01-01")
        mock_forecast.median =  np.array([[100, 200], [110, 220]])
        
        # Define confidence interval
        confidence_interval = {'min': 0.05, 'max': 0.95}

        # Define mock return values for min and max quantiles (2x2 arrays)
        mock_min_quantiles = np.array([[95, 190], [105, 210]])
        mock_max_quantiles = np.array([[105, 210], [115, 230]])

        # Configure mock quantile method to return corresponding values based on input
        mock_forecast.quantile.side_effect = lambda quantile: (
            mock_min_quantiles if quantile == confidence_interval['min'] else mock_max_quantiles
        )

        mock_forecast.freq = "1H"
        
        mock_predictor.predict.return_value = [mock_forecast]
        inference.model_dict["models"]["algorithm"]["config"] = mock_predictor
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0], [1672534800.0, "dev_1", 95.0, 190.0]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            },
            "dev_2": {
                "data": [[1672531200.0, "dev_2", 100.0, 200.0], [1672534800.0, "dev_2", 105.0, 210.0]],
                "confidence_interval": {"min": 0.2, "max": 0.8},
            }
        })
        
        result = inference.predict("algorithm", "config", input_data)
        
        self.assertIn("dev_1", result)
        self.assertIn("predictions", result["dev_1"])
        self.assertIn("min_boundary", result["dev_1"])
        self.assertIn("max_boundary", result["dev_1"])
        self.assertEqual(len(result["dev_1"]["predictions"]), 2)
        self.assertEqual(len(result["dev_1"]["min_boundary"]), 2)
        self.assertEqual(len(result["dev_1"]["max_boundary"]), 2)
        
        mock_predictor.predict.assert_called_once()
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_predict_ml_algorithm_not_found(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"existing_algorithm": {"config": MagicMock()}},
            "params": {"existing_algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672617600.0, "dev_1", 2000.0, 10.0]],
                "confidence_interval": {"min": 0.6, "max": 0.8},
            }
        })
        
        with self.assertRaises(HedgeInferenceException) as context:
            inference.predict("non_existing_algorithm", "config", input_data)
        
        self.assertEqual(context.exception.status_code, 404)
        self.assertEqual(str(context.exception.detail), "No model found")  
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    @patch('gluonts.dataset.common.ListDataset')
    def test_predict_invalid_frequency(self, mock_list_dataset, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        mock_predictor = MagicMock()
        mock_predictor.predict.side_effect = ValueError("Invalid frequency")
        inference.model_dict["models"]["algorithm"]["config"] = mock_predictor
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0], [1672534800.0, "dev_1", 95.0, 190.0]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            }
        })
        
        with self.assertRaises(HedgeInferenceException) as context:
            inference.predict("algorithm", "config", input_data)
        
        self.assertEqual(context.exception.status_code, 500)
        self.assertTrue("Invalid frequency" in str(context.exception.detail))
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    @patch('gluonts.dataset.common.ListDataset')
    def test_predict_multiple_target_columns(self, mock_list_dataset, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2", "target3"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2", "target3"],
                "output_columns": ["timestamp", "device", "target1", "target2", "target3"]
            }}}
        }
        
        mock_predictor = MagicMock()
        mock_forecast = MagicMock()
        mock_forecast.item_id = "dev_1"
        mock_forecast.start_date.to_timestamp.return_value = pd.Timestamp("2023-01-01")
        mock_forecast.median = np.array([[100, 200, 300], [110, 220, 330]])
        
        # Define confidence interval
        confidence_interval = {'min': 0.05, 'max': 0.95}

        # Define mock return values for min and max quantiles (2x2 arrays)
        mock_min_quantiles = np.array([[95, 190, 285], [105, 210, 315]])
        mock_max_quantiles = np.array([[105, 210, 315], [115, 230, 460]])

        # Configure mock quantile method to return corresponding values based on input
        mock_forecast.quantile.side_effect = lambda quantile: (
            mock_min_quantiles if quantile == confidence_interval['min'] else mock_max_quantiles
        )
        mock_forecast.freq = "1H"
        
        mock_predictor.predict.return_value = [mock_forecast]
        inference.model_dict["models"]["algorithm"]["config"] = mock_predictor
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0, 270.0], [1672534800.0, "dev_1", 95.0, 190.0, 285.0]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            }
        })
        
        result = inference.predict("algorithm", "config", input_data)
        
        self.assertIn("dev_1", result)
        self.assertIn("predictions", result["dev_1"])
        self.assertIn("min_boundary", result["dev_1"])
        self.assertIn("max_boundary", result["dev_1"])
        
        predictions = result["dev_1"]["predictions"]
        self.assertEqual(len(predictions), 2)
        self.assertEqual(len(predictions[0]), 5)  # timestamp, device, target1, target2, target3
        
        self.assertAlmostEqual(predictions[0][2], 100.0, places=1)  # target1
        self.assertAlmostEqual(predictions[0][3], 200.0, places=1)  # target2
        self.assertAlmostEqual(predictions[0][4], 300.0, places=1)  # target3
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    @patch('gluonts.dataset.common.ListDataset')
    def test_predict_gluonts_prediction_failure(self, mock_list_dataset, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        mock_predictor = MagicMock()
        mock_predictor.predict.side_effect = Exception("GluonTS prediction failed")
        inference.model_dict["models"]["algorithm"]["config"] = mock_predictor
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0], [1672534800.0, "dev_1", 95.0, 190.0]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            }
        })
        
        with self.assertRaises(HedgeInferenceException) as context:
            inference.predict("algorithm", "config", input_data)
        
        self.assertEqual(context.exception.status_code, 500)
        self.assertTrue("Prediction failed with error: GluonTS prediction failed" in str(context.exception.detail))
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    @patch('gluonts.dataset.common.ListDataset')
    def test_predict_rounding(self, mock_list_dataset, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        mock_predictor = MagicMock()
        mock_forecast = MagicMock()
        mock_forecast.item_id = "dev_1"
        mock_forecast.start_date.to_timestamp.return_value = pd.Timestamp("2023-01-01")
        mock_forecast.median = np.array([[100.123456, 200.987654], [110.543210, 220.948765]])
        mock_forecast.samples = np.array([[[95.123456, 190.987654], [105.543210, 210.938765]]])
        # Define confidence interval
        confidence_interval = {'min': 0.05, 'max': 0.95}
    
        # Define mock return values for min and max quantiles (2x2 arrays)
        mock_min_quantiles = np.array([[95.123456, 190.987654], [105, 210.938765]])
        mock_max_quantiles = np.array([[105.96765, 210.98465], [115.934665, 230.948665]])

        # Configure mock quantile method to return corresponding values based on input
        mock_forecast.quantile.side_effect = lambda quantile: (
            mock_min_quantiles if quantile == confidence_interval['min'] else mock_max_quantiles
        )
        mock_forecast.freq = "1H"
        
        mock_predictor.predict.return_value = [mock_forecast]
        inference.model_dict["models"]["algorithm"]["config"] = mock_predictor
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.05, 180.06]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            }
        })
        
        result = inference.predict("algorithm", "config", input_data)
        
        self.assertEqual(len(result["dev_1"]["predictions"]), 2)
        self.assertEqual(len(result["dev_1"]["min_boundary"]), 2)
        self.assertEqual(len(result["dev_1"]["max_boundary"]), 2)
        
        for prediction in result["dev_1"]["predictions"]:
            self.assertEqual(len(str(prediction[2]).split('.')[1]), 2)  # Check target1
            self.assertEqual(len(str(prediction[3]).split('.')[1]), 2)  # Check target2
        
        for min_bound in result["dev_1"]["min_boundary"]:
            self.assertEqual(len(str(min_bound[2]).split('.')[1]), 2)  # Check target1
            self.assertEqual(len(str(min_bound[3]).split('.')[1]), 2)  # Check target2
        
        for max_bound in result["dev_1"]["max_boundary"]:
            self.assertEqual(len(str(max_bound[2]).split('.')[1]), 2)  # Check target1
            self.assertEqual(len(str(max_bound[3]).split('.')[1]), 2)  # Check target2
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_validate_confidence_interval_min_less_than_or_equal_to_zero(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {"min": 0.0, "max": 0.8}
        
        with self.assertRaises(ValueError) as context:
            inference.validate_confidence_interval(confidence_interval)
        
        self.assertEqual(str(context.exception), "'min' value (0.0) must be between 0.0 and 1.0, exclusive.")
    
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    def test_validate_confidence_interval_max_greater_than_or_equal_to_one(self, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {"min": 0.5, "max": 1.0}
        
        with self.assertRaises(ValueError) as context:
            inference.validate_confidence_interval(confidence_interval)
        
        self.assertEqual(str(context.exception), "'max' value (1.0) must be between 0.0 and 1.0, exclusive.")
    
    def test_validate_confidence_interval_min_equal_max(self):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {"min": 0.5, "max": 0.5}
        
        with self.assertRaises(ValueError) as context:
            inference.validate_confidence_interval(confidence_interval)
        
        self.assertEqual(
            str(context.exception),
            "'min' value (0.5) must be less than 'max' value (0.5)."
        )
    
    def test_validate_confidence_interval_min_greater_than_max(self):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {"min": 0.8, "max": 0.6}
        
        with self.assertRaises(ValueError) as context:
            inference.validate_confidence_interval(confidence_interval)
        
        self.assertEqual(str(context.exception), "'min' value (0.8) must be less than 'max' value (0.6).")
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_validate_confidence_interval_missing_min(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {"max": 0.8}
        
        with self.assertRaises(ValueError) as context:
            inference.validate_confidence_interval(confidence_interval)
        
        self.assertEqual(str(context.exception), "Both 'min' and 'max' must be provided in the confidence interval.")
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_validate_confidence_interval_non_numeric_values(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {"min": "0.1", "max": "0.9"}
        
        with self.assertRaises(ValueError) as context:
            inference.validate_confidence_interval(confidence_interval)
        
        self.assertTrue("'min' and 'max' must be valid numeric values (float or int)." in str(context.exception))
        
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_validate_confidence_interval_with_unexpected_keys(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        confidence_interval = {
            "min": 0.1,
            "max": 0.9,
            "unexpected_key": "unexpected_value"
        }
        
        # The method should not raise an exception for unexpected keys
        try:
            inference.validate_confidence_interval(confidence_interval)
        except ValueError:
            self.fail("validate_confidence_interval() raised ValueError unexpectedly!")
        
        # Verify that the method still validates the required keys correctly
        with self.assertRaises(ValueError):
            invalid_confidence_interval = {
                "min": 1.1,  # Invalid value
                "max": 0.9,
                "unexpected_key": "unexpected_value"
            }
            inference.validate_confidence_interval(invalid_confidence_interval)
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.EnvironmentUtil')
    def test_validate_multiple_confidence_intervals(self, mock_env_util, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        
        valid_intervals = [
            {"min": 0.1, "max": 0.9},
            {"min": 0.25, "max": 0.75},
            {"min": 0.4, "max": 0.6}
        ]
        
        for interval in valid_intervals:
            try:
                inference.validate_confidence_interval(interval)
            except ValueError:
                self.fail(f"validate_confidence_interval raised ValueError unexpectedly for interval: {interval}")
        
        invalid_intervals = [
            {"min": 0, "max": 1},
            {"min": 0.5, "max": 0.5},
            {"min": 0.7, "max": 0.6},
            {"min": -0.1, "max": 1.1},
            {"min": 0.3, "max": None},
            {"max": 0.8}
        ]
        
        for interval in invalid_intervals:
            with self.assertRaises(ValueError):
                inference.validate_confidence_interval(interval)
    
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    def test_default_confidence_interval(self, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0]]
            }
        })
        
        inference.predict("algorithm", "config", input_data)
        
        self.assertEqual(input_data.__root__["dev_1"]["confidence_interval"], {"min": 0.25, "max": 0.75})
        mock_logger_util().logger.info.assert_any_call("No confidence interval found for correlation ID: dev_1")
        mock_logger_util().logger.info.assert_any_call("Default confidence interval set: {'min': 0.25, 'max': 0.75}")
    
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    def test_handle_multiple_correlation_ids_with_missing_confidence_intervals(self, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            },
            "dev_2": {
                "data": [[1672531200.0, "dev_2", 100.0, 200.0]],
            },
            "dev_3": {
                "data": [[1672531200.0, "dev_3", 110.0, 220.0]],
            }
        })
        
        with patch.object(inference, 'validate_confidence_interval') as mock_validate:
            inference.predict("algorithm", "config", input_data)
        
        self.assertEqual(input_data.__root__["dev_2"]["confidence_interval"], {"min": 0.25, "max": 0.75})
        self.assertEqual(input_data.__root__["dev_3"]["confidence_interval"], {"min": 0.25, "max": 0.75})
        
        mock_logger_util.return_value.logger.info.assert_any_call("Confidence interval found for correlation ID: dev_1")
        mock_logger_util.return_value.logger.info.assert_any_call("No confidence interval found for correlation ID: dev_2")
        mock_logger_util.return_value.logger.info.assert_any_call("No confidence interval found for correlation ID: dev_3")
        mock_logger_util.return_value.logger.info.assert_any_call("Default confidence interval set: {'min': 0.25, 'max': 0.75}")
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    def test_handle_empty_confidence_interval(self, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672617600.0, "dev_1", 2000.0, 10.0]],
                "confidence_interval": {},
            }
        })
        
        with patch.object(inference, 'validate_confidence_interval') as mock_validate:
            inference.predict("algorithm", "config", input_data)
        
        #mock_validate.assert_called_once_with({"min": 0.25, "max": 0.75})
        mock_logger_util().logger.info.assert_any_call("No confidence interval found for correlation ID: dev_1")
        mock_logger_util().logger.info.assert_any_call("Default confidence interval set: {'min': 0.25, 'max': 0.75}")
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    def test_confidence_interval_present_but_empty(self, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0]],
                "confidence_interval": {},
            },
            "dev_2": {
                "data": [[1672531200.0, "dev_2", 100.0, 200.0]],
                "confidence_interval": {"min": 0.2, "max": 0.8},
            }
        })
        
        with patch.object(inference, 'validate_confidence_interval') as mock_validate:
            inference.predict("algorithm", "config", input_data)
        
        mock_logger_util.return_value.logger.info.assert_any_call("No confidence interval found for correlation ID: dev_1")
        mock_logger_util.return_value.logger.info.assert_any_call("Default confidence interval set: {'min': 0.25, 'max': 0.75}")
        mock_logger_util.return_value.logger.info.assert_any_call("Confidence interval found for correlation ID: dev_2")
    
    @patch('timeseries.multivariate_deepVAR.infer.src.main.task.LoggerUtil')
    def test_mixed_confidence_intervals(self, mock_logger_util):
        inference = TimeseriesDeepVARInference()
        inference.model_dict = {
            "models": {"algorithm": {"config": MagicMock()}},
            "params": {"algorithm": {"config": {
                "target_columns": ["target1", "target2"],
                "group_by_cols": ["device"],
                "freq": "1H",
                "date_field": "timestamp",
                "input_columns": ["timestamp", "device", "target1", "target2"],
                "output_columns": ["timestamp", "device", "target1", "target2"]
            }}}
        }
        
        input_data = TimeseriesInputs(__root__={
            "dev_1": {
                "data": [[1672531200.0, "dev_1", 90.0, 180.0]],
                "confidence_interval": {"min": 0.1, "max": 0.9},
            },
            "dev_2": {
                "data": [[1672531200.0, "dev_2", 100.0, 200.0]],
            }
        })
        
        with patch.object(inference, 'validate_confidence_interval') as mock_validate:
            inference.predict("algorithm", "config", input_data)
        
        mock_logger_util.return_value.logger.info.assert_any_call("Confidence interval found for correlation ID: dev_1")
        mock_logger_util.return_value.logger.info.assert_any_call("No confidence interval found for correlation ID: dev_2")
        mock_logger_util.return_value.logger.info.assert_any_call("Default confidence interval set: {'min': 0.25, 'max': 0.75}")
        
        self.assertEqual(input_data.__root__["dev_2"]["confidence_interval"], {"min": 0.25, "max": 0.75})
    
if __name__ == '__main__':
    unittest.main()
