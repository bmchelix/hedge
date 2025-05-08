import json
import unittest
from unittest.mock import MagicMock, mock_open, patch
from warnings import filterwarnings
from common.src.data.hedge_util import HedgeUtil, DataSourceInfo
import numpy as np
import pandas as pd

from timeseries.multivariate_deepVAR.train.src.main.task import TimeSeriesForecasting
from common.src.util.exceptions import HedgeTrainingException

filterwarnings(action="ignore", category=DeprecationWarning)


class TestTimeSeriesForecasting(unittest.TestCase):
    def setUp(self):
        with patch.dict("os.environ", {"LOCAL": "True"}):
            self.forecasting = TimeSeriesForecasting()

    def test_split_train_validation_datasets(self):
        # Create a sample DataFrame
        data = pd.DataFrame(
            {
                "date": pd.date_range(start="2023-01-01", periods=100, freq="H"),
                "deviceName": ["device1"] * 50 + ["device2"] * 50,
                "siteID": ["site1"] * 50 + ["site2"] * 50,
                "target1": np.random.rand(100),
                "target2": np.random.rand(100),
            }
        )
        data["date"] = (data["date"] - pd.Timestamp("1970-01-01")) // pd.Timedelta("1s")

        data["date"] = pd.to_datetime(data["date"], unit="s")
        data.sort_values(by="date", inplace=True)
        # Initialize TimeSeriesForecasting instance
        forecasting = self.forecasting
        forecasting.date_field = "date"

        # Call the method
        train_ds, val_ds = forecasting.split_train_validation_datasets(
            data=data,
            groupby_columns=["deviceName", "siteID"],
            target_columns=["target1", "target2"],
            time_stamp_column="date",
            split_ratio=0.8,
        )

        # Check that datasets are of correct type
        # self.assertIsInstance(train_ds, type(ListDataset), "Instances are ListDataset type")
        # self.assertIsInstance(val_ds, type(ListDataset), "Instances are ListDataset type")
        self.assertTrue(type(train_ds).__name__, "ListDataset")
        self.assertTrue(type(val_ds).__name__, "ListDataset")

        # Check that the correct number of items are in each dataset
        self.assertEqual(len(list(train_ds)), 2)  # 2 unique item_ids
        self.assertEqual(len(list(val_ds)), 2)  # 2 unique item_ids

        # Check that the split ratio is correct
        train_item = next(iter(train_ds))
        val_item = next(iter(val_ds))
        self.assertEqual(train_item["target"].shape[1], 40)  # 80% of 50
        self.assertEqual(val_item["target"].shape[1], 10)  # 20% of 50

        # Check that item_ids are preserved
        train_item_ids = set(item["item_id"] for item in train_ds)
        val_item_ids = set(item["item_id"] for item in val_ds)
        self.assertEqual(train_item_ids, val_item_ids)
        self.assertEqual(train_item_ids, {"device1_site1", "device2_site2"})

    def test_infer_correct_frequency(self):
        # Create a sample DataFrame with timestamp data
        data = pd.DataFrame(
            {
                "date": pd.date_range(start="2023-01-01", end="2023-01-10", freq="D"),
                "deviceName": ["device1"] * 10,
                "siteID": ["site1"] * 10,
                "target1": range(10),
                "target2": range(10, 20),
            }
        )
        data["date"] = data["date"].astype(int) // 10**9  # Convert to Unix timestamp
        data["date"] = pd.to_datetime(data["date"], unit="s")
        data.sort_values(by="date", inplace=True)

        with patch.dict("os.environ", {"LOCAL": "True"}):
            # Initialize TimeSeriesForecasting instance
            tsf = TimeSeriesForecasting()
            tsf.date_field = "date"
            tsf.group_by_cols = ["deviceName", "siteID"]
            tsf.target_columns = ["target1", "target2"]

            # Call the split_train_validation_datasets method
            train_ds, val_ds = tsf.split_train_validation_datasets(
                data=data,
                groupby_columns=tsf.group_by_cols,
                target_columns=tsf.target_columns,
                time_stamp_column=tsf.date_field,
            )

            # Assert that the inferred frequency is correct
            self.assertEqual(tsf.freq, "D")


    @patch('common.src.data.hedge_util.os.makedirs')
    @patch('common.src.data.hedge_util.zipfile.ZipFile')
    @patch('common.src.data.hedge_util.glob.glob')
    def test_read_data(self, mock_glob, mock_zipfile, mock_makedirs):
        # Create a mock HedgeUtil instance
        hedge_util = MagicMock(spec=HedgeUtil)
        hedge_util.model_dir = '/mock/model/dir'
        hedge_util.training_file_id = 'mock_training_file_id'
        hedge_util.logger = MagicMock()

        # Mock the abstract methods
        hedge_util.set_model_dir.return_value = '/mock/training/model/dir'
        hedge_util.download_data.return_value = '/mock/zip/file.zip'

        # Mock the check_data_folder_paths method
        hedge_util.check_data_folder_paths.return_value = ['algorithm', 'version']

        # Set up glob mock
        mock_glob.side_effect = [
            ['/mock/training/model/dir/algorithm/version/data/data.csv'],
            ['/mock/training/model/dir/algorithm/version/data/config.json']
        ]

        # Call the read_data method
        result = HedgeUtil.read_data(hedge_util)

        # Assert the result
        self.assertIsInstance(result, DataSourceInfo)
        self.assertEqual(result.csv_file_path, '/mock/training/model/dir/algorithm/version/data/data.csv')
        self.assertEqual(result.algo_name, 'algorithm')
        self.assertEqual(result.config_file_path, '/mock/training/model/dir/algorithm/version/data/config.json')

        # Verify that the mocked methods were called
        hedge_util.set_model_dir.assert_called_once_with('/mock/model/dir', 'mock_training_file_id')
        hedge_util.download_data.assert_called_once()
        hedge_util.check_data_folder_paths.assert_called_once_with('/mock/training/model/dir')
        self.assertEqual(mock_glob.call_count, 2)
        mock_makedirs.assert_called_once_with('/mock/training/model/dir', exist_ok=True)

    @patch('common.src.data.hedge_util.HedgeUtil.read_data')
    def test_missing_timestamp_field(self, mock_read_data):
        # Mock the config.json with missing group by field
        mock_config = {
            "input_features": [
                {"name": "timestamp", "type": "date"},
                {"name": "target1", "type": "numerical"},
                {"name": "target2", "type": "numerical"},
            ],
            "output_features": [
                {"name": "target1", "type": "numerical"},
                {"name": "target2", "type": "numerical"},
            ],
            "timestamp": "timestamp",
            "prediction_length": 24,
        }

        # Mock the CSV data
        mock_csv_data = pd.DataFrame(
            {
                "timestamp": pd.date_range(start="2023-01-01", periods=100, freq="H"),
                "target1": np.random.rand(100),
                "target2": np.random.rand(100),
            }
        )
        
        # Set up the mock for read_data
        mock_read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/to/config.json",
            csv_file_path="/mock/path/to/data.csv"
        )

        with patch.dict("os.environ", {"LOCAL": "True", "TRAINING_FILE_ID": "training.zip"}):
            # Initialize TimeSeriesForecasting instance
            self.time_series_forecasting = TimeSeriesForecasting()
            # Mock necessary methods and attributes
            self.time_series_forecasting.feature_extractor = MagicMock()
            self.time_series_forecasting.feature_extractor.get_group_by_cols.return_value = None
            self.time_series_forecasting.feature_extractor.get_timestamp_field.return_value = None

            # Mock open function to return mock config
            with patch("builtins.open", mock_open(read_data=json.dumps(mock_config))):
                # Mock pd.read_csv to return mock CSV data
                with patch("pandas.read_csv", return_value=mock_csv_data):
                    with self.assertRaises(HedgeTrainingException) as context:
                        self.time_series_forecasting.train()
                    self.assertIn("Timestamp field is missing or incorrectly specified in the config.json", str(context.exception))

    @patch('common.src.ml.hedge_status.HedgePipelineStatus.update')
    @patch('common.src.data.hedge_util.HedgeUtil.read_data')
    @patch("common.src.util.config_extractor.FeatureExtractor")
    def test_group_by_field_missing(self, mock_feature_extractor, mock_read_data, mock_hedge_pipeline_status):
        # Mock the config.json with missing group by field
        mock_config = {
            "mlModelConfig": {
                "mlDataSourceConfig": {
                    "featuresByProfile": {
                        "WindTurbine": [
                            {
                                "type": "METRIC",
                                "name": "TurbinePower",
                                "isInput": True,
                                "isOutput": True,
                            }
                        ]
                    },
                    "featureNameToColumnIndex": {
                        "Timestamp": 0,
                        "WindTurbine#TurbinePower": 2,
                        "deviceName": 1,
                    },
                }
            },
            "mlAlgoDefinition": {
                "timeStampAttributeRequired": True,
                "groupByAttributesRequired": False,
            },
        }

        # Mock the CSV data
        mock_csv_data = pd.DataFrame(
            {
                "Timestamp": pd.date_range(start="2023-01-01", periods=100, freq="H"),
                "target1": np.random.rand(100),
                "target2": np.random.rand(100),
            }
        )

        # Set up the mock for read_data
        mock_read_data.return_value = MagicMock(
            algo_name="mock_algo",
            config_file_path="/mock/path/to/config.json",
            csv_file_path="/mock/path/to/data.csv"
        )
        
        # Mock HedgePipelineStatus
        mock_status_instance = MagicMock()
        mock_hedge_pipeline_status.return_value = mock_status_instance


        # Mock the FeatureExtractor to return an empty list for group_by_cols
        mock_feature_extractor.return_value.get_group_by_cols.return_value = []

        with patch.dict("os.environ", {"LOCAL": "True", "TRAINING_FILE_ID": "training.zip", "MODELDIR": ""}):
            # Initialize TimeSeriesForecasting instance
            time_series_forecasting = TimeSeriesForecasting()
            
            # Mock necessary methods and attributes
            time_series_forecasting.feature_extractor = mock_feature_extractor.return_value
            time_series_forecasting.feature_extractor.timestamp = "timestamp"
            time_series_forecasting.feature_extractor.get_input_features_list.return_value = ["timestamp", "target1", "target2"]
            time_series_forecasting.feature_extractor.get_numerical_inputs_list.return_value = ["target1", "target2"]
            time_series_forecasting.feature_extractor.get_output_features_list.return_value = ["target1", "target2"]

            # Mock open function to return mock config
            with patch("builtins.open", mock_open(read_data=json.dumps(mock_config))):
                # Mock pd.read_csv to return mock CSV data
                with patch("pandas.read_csv", return_value=mock_csv_data):
                    with self.assertRaises(HedgeTrainingException) as context:
                        time_series_forecasting.train()
                    self.assertIn("Group By field is missing or incorrectly specified in the config.json", str(context.exception))            
        
    def test_remove_inconsistent_records_identical_timestamps(self):
        # Create a sample DataFrame with identical timestamps
        data = pd.DataFrame({
            'date': pd.to_datetime(['2023-01-01 00:00:00'] * 5),
            'value': range(5)
        })
        
        forecasting = self.forecasting
        result = forecasting._remove_inconsistent_records(data, 'date')
        
        # Check that all rows are retained
        self.assertEqual(len(result), 5)
        
        # Check that the 'time_diff' column is not present in the result
        self.assertNotIn('time_diff', result.columns)
        
        # Check that the DataFrame is unchanged (except for index reset)
        pd.testing.assert_frame_equal(result.reset_index(drop=True), data.reset_index(drop=True))
    
    def test_remove_inconsistent_records_non_consecutive_timestamps(self):
        # Create a sample DataFrame with non-consecutive timestamps
        data = pd.DataFrame({
            'date': pd.to_datetime(['2023-01-01', '2023-01-02', '2023-01-03', '2023-01-05', '2023-01-06', '2023-01-08', '2023-01-09']),
            'value': [1, 2, 3, 4, 5, 6, 7]
        })
    
        # Initialize TimeSeriesForecasting instance
        forecasting = self.forecasting
    
        # Call the method
        cleaned_df = forecasting._remove_inconsistent_records(data, 'date')
    
        # Check that the correct number of rows are in the cleaned DataFrame
        self.assertEqual(len(cleaned_df), 3)
    
        # Check that the values are correct
        expected_values = [1, 2, 3]
        self.assertListEqual(cleaned_df['value'].tolist(), expected_values)
        
    
    def test_remove_inconsistent_records(self):
        # Create a sample DataFrame with inconsistent time differences
        data = pd.DataFrame({
            'date': [
                '2023-01-01 00:00:00', '2023-01-01 01:00:00', '2023-01-01 02:00:00',
                '2023-01-01 03:00:00', '2023-01-01 05:00:00', '2023-01-01 06:00:00',
                '2023-01-01 07:00:00', '2023-01-01 09:00:00', '2023-01-01 10:00:00'
            ],
            'value': range(9)
        })
        data['date'] = pd.to_datetime(data['date'])
    
        # Initialize TimeSeriesForecasting instance
        forecasting = self.forecasting
    
        # Call the method
        cleaned_df = forecasting._remove_inconsistent_records(data, 'date')
    
        # Check that the correct number of records are retained
        self.assertEqual(len(cleaned_df), 4)
    
        # Check that the retained records have consistent time differences
        time_diffs = cleaned_df['date'].diff()[1:]
        self.assertTrue(all(time_diffs == pd.Timedelta(hours=1)))
    
        # Check that the correct records are retained
        expected_values = [0, 1, 2, 3]
        self.assertTrue(all(cleaned_df['value'] == expected_values))

    
    def test_remove_inconsistent_records_non_unique_most_common_diff(self):
        # Create a sample DataFrame with non-unique most common time difference
        data = pd.DataFrame({
            'date': pd.to_datetime(['2023-01-01 00:00:00', '2023-01-01 01:00:00', 
                                    '2023-01-01 02:00:00', '2023-01-01 04:00:00', 
                                    '2023-01-01 05:00:00', '2023-01-01 07:00:00']),
            'value': [1, 2, 3, 4, 5, 6]
        })
    
        # Initialize TimeSeriesForecasting instance
        forecasting = self.forecasting
    
        # Call the method
        cleaned_df = forecasting._remove_inconsistent_records(data, 'date')
    
        # Check that the cleaned DataFrame has the correct number of rows
        self.assertEqual(len(cleaned_df), 3)
    
        # Check that the cleaned DataFrame has the correct time differences
        time_diffs = cleaned_df['date'].diff().dropna()
        self.assertTrue(all(time_diff == pd.Timedelta(hours=1) for time_diff in time_diffs))
    
        # Check that the correct rows are retained
        expected_values = [1, 2, 3]
        self.assertTrue(all(cleaned_df['value'] == expected_values))
    
    
    def test_remove_inconsistent_records_with_missing_values(self):
        # Create a sample DataFrame with missing values in datetime column
        data = pd.DataFrame({
            'date': ['2023-01-01', '2023-01-02', '2023-01-03', None, '2023-01-06', '2023-01-07'],
            'value': [1, 2, 3, 4, 5, 6]
        })
        
        # Initialize TimeSeriesForecasting instance
        tsf = self.forecasting
        
        # Call the method
        cleaned_df = tsf._remove_inconsistent_records(data, 'date')
        
        # Check that rows with missing datetime values are dropped
        self.assertEqual(len(cleaned_df), 3)
        
        # Check that the 'time_diff' column is not present in the final DataFrame
        self.assertNotIn('time_diff', cleaned_df.columns)
    
    
    def test_remove_inconsistent_records_all_inconsistent(self):
        # Create a sample DataFrame with inconsistent timestamps
        data = pd.DataFrame({
            'date': pd.to_datetime(['2023-01-01 00:00:00', '2023-01-01 00:05:00', '2023-01-01 00:15:00', '2023-01-01 00:30:00']),
            'value': [1, 2, 3, 4]
        })
    
        # Initialize TimeSeriesForecasting instance
        tsf = self.forecasting
    
        # Call the method
        result = tsf._remove_inconsistent_records(data, 'date')
        
        print(result.head())
    
        # Check that the result is an empty DataFrame
        self.assertTrue(result.empty)
        self.assertEqual(len(result), 0)
    
    
    def test_remove_inconsistent_records_largest_block_at_end(self):
        # Create a sample DataFrame with inconsistent time differences
        data = pd.DataFrame({
            'date': pd.to_datetime([
                '2023-01-01 00:00:00', '2023-01-01 01:00:00', '2023-01-01 03:00:00',
                '2023-01-01 04:00:00', '2023-01-01 05:00:00', '2023-01-01 06:00:00'
            ]),
            'value': [1, 2, 3, 4, 5, 6]
        })
    
        # Initialize TimeSeriesForecasting instance
        forecasting = self.forecasting
    
        # Call the method
        cleaned_df = forecasting._remove_inconsistent_records(data, 'date')
    
        # Check that the largest consistent block (last 4 rows) is retained
        expected_dates = pd.to_datetime(['2023-01-01 03:00:00', '2023-01-01 04:00:00', '2023-01-01 05:00:00', '2023-01-01 06:00:00'])
        
        # Convert the cleaned_df['date'] Series to an Index
        pd.testing.assert_index_equal(
            pd.Index(cleaned_df['date'].values), 
            pd.Index(expected_dates)
        )
        
        # Check that the values correspond to the retained dates
        np.testing.assert_array_equal(cleaned_df['value'], [3, 4, 5, 6])

if __name__ == "__main__":
    unittest.main()
