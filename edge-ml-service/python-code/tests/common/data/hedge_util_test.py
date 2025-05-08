"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import unittest
from unittest.mock import MagicMock, patch, mock_open
import os
import zipfile
from common.src.data.hedge_util import HedgeUtil, DataSourceInfo

class TestHedgeUtil(unittest.TestCase):

    @patch('os.walk')
    @patch('common.src.util.logger_util.LoggerUtil')
    def test_check_data_folder_paths(self, mock_logger_util, mock_os_walk):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger
        mock_os_walk.return_value = [
            ("root", ["folder1", "folder2"], ["file1"])
        ]

        class TestUtil(HedgeUtil):
            def __init__(self):
                super().__init__()

            def download_data(self): pass

            def upload_data(self, model_zip_file): pass

            def set_model_dir(self, model_dir, training_file_id): pass

            def update_status(self, success, message): pass

        util = TestUtil()
        util.logger = mock_logger

        folder_names = util.check_data_folder_paths("test_dir")

        self.assertEqual(folder_names, ["folder1", "folder2"])
        mock_logger.info.assert_called_once_with("Folder Names: ['folder1', 'folder2']")

    @patch('zipfile.ZipFile')
    @patch('common.src.util.logger_util.LoggerUtil')
    def test_unzip_data(self, mock_logger_util, mock_zipfile):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger

        class TestUtil(HedgeUtil):
            def __init__(self):
                super().__init__()

            def download_data(self): pass

            def upload_data(self, model_zip_file): pass

            def set_model_dir(self, model_dir, training_file_id): pass

            def update_status(self, success, message): pass

        util = TestUtil()
        util.logger = mock_logger
        util.model_dir = "test_model_dir"

        mock_zip = mock_zipfile.return_value.__enter__.return_value
        mock_zip.namelist.return_value = ['file1.txt', '__MACOSX/file2.txt']

        util._HedgeUtil__unzip_data("test.zip")

        mock_zip.extract.assert_called_once_with('file1.txt', "test_model_dir")
        mock_logger.info.assert_called_once_with('Unzipped test.zip successfully without __MACOSX')

    @patch('os.makedirs')
    @patch('zipfile.ZipFile')
    @patch('glob.glob')
    @patch('common.src.util.logger_util.LoggerUtil')
    @patch('common.src.data.hedge_util.HedgeUtil.check_data_folder_paths', return_value=["folder1", "folder2"])
    def test_read_data(self, mock_check_paths, mock_logger_util, mock_glob, mock_zipfile, mock_makedirs):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger

        class TestUtil(HedgeUtil):
            def __init__(self):
                super().__init__()

            def download_data(self): return "test.zip"

            def upload_data(self, model_zip_file): pass

            def set_model_dir(self, model_dir, training_file_id):
                return "test_model_dir"

            def update_status(self, success, message): pass

        mock_glob.side_effect = lambda path: ["test.csv"] if "*.csv" in path else ["test.json"]
        mock_zip = mock_zipfile.return_value.__enter__.return_value
        mock_zip.namelist.return_value = ["folder1/folder2/data/test.csv", "folder1/folder2/data/test.json"]

        util = TestUtil()
        util.logger = mock_logger
        util.model_dir = "test_model_dir"
        util.training_file_id = "1234"

        data_source_info = util.read_data()

        self.assertEqual(data_source_info.csv_file_path, "test_model_dir/folder1/folder2/data/test.csv")
        self.assertEqual(data_source_info.config_file_path, "test_model_dir/folder1/folder2/data/test.json")
        self.assertEqual(data_source_info.algo_name, "folder1")

        mock_logger.info.assert_any_call("zip-file: test.zip")
        mock_logger.info.assert_any_call("model_dir: test_model_dir")
        mock_logger.info.assert_any_call("Fully Qualified csv Path: test_model_dir/folder1/folder2/data/test.csv")
        mock_logger.info.assert_any_call(
            "Fully Qualified config json Path: test_model_dir/folder1/folder2/data/test.json")

    @patch('common.src.data.hedge_util.LoggerUtil')
    def test_init_logger(self, mock_logger_util):
        mock_logger = MagicMock()
        mock_logger_util.return_value.logger = mock_logger

        class TestUtil(HedgeUtil):
            def __init__(self):
                super().__init__()

            def download_data(self): pass

            def upload_data(self, model_zip_file): pass

            def set_model_dir(self, model_dir, training_file_id): pass

            def update_status(self, success, message): pass

        util = TestUtil()

        self.assertIs(util.logger, mock_logger_util.return_value.logger)


if __name__ == "__main__":
    unittest.main()
