"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from abc import ABC, abstractmethod

import os
import glob
import zipfile
from pydantic import BaseModel
from common.src.util.logger_util import LoggerUtil
from logging import Logger


class DataSourceInfo(BaseModel):
    csv_file_path: str
    algo_name: str
    config_file_path: str

class HedgeUtil(ABC):
    
    model_dir: str = ""
    training_file_id: str = "" 
    logger: Logger    
    
    @abstractmethod
    def __init__(self):
        self.logger = LoggerUtil().logger
        pass    
    
    @abstractmethod
    def download_data(self) -> str:
        pass
    
    @abstractmethod
    def upload_data(self, model_zip_file):
        pass

    def __unzip_data(self, input_zip_file):
        with zipfile.ZipFile(input_zip_file, "r") as zip_ref:
            for member in zip_ref.namelist():
                if '__MACOSX' not in member:
                    zip_ref.extract(member, self.model_dir)
        self.logger.info(f'Unzipped {input_zip_file} successfully without __MACOSX')              


    def check_data_folder_paths(self, directory):
        folder_names = []
        for root, dirs, files in os.walk(directory):
            folder_names.extend(dirs)

        self.logger.info(f"Folder Names: {folder_names}")
        return folder_names
    
    @abstractmethod
    def set_model_dir(self, model_dir, training_file_id) -> str:
        pass
    

    def read_data(self) -> DataSourceInfo:
        
        training_model_dir = self.set_model_dir(self.model_dir, self.training_file_id)
        self.logger.info(f'{training_model_dir=}')

        # Create directory if not found
        os.makedirs(training_model_dir, exist_ok=True)
        
        # # Remove contents inside the directory if any files present
        # shutil.rmtree(training_model_dir, ignore_errors=False, onerror=None)
        
        # Download the zip-file
        zip_file = self.download_data()

        self.logger.info(f"zip-file: {zip_file}") 
        if "FAILED" != zip_file:
            # Unzip the file if download was successful
            self.__unzip_data(zip_file)
        else:
            raise ValueError(f"Error downloading zipfile, {self.training_file_id}")           
        
        self.logger.info(f"model_dir: {training_model_dir}")
        subfolders = self.check_data_folder_paths(training_model_dir)

        self.logger.info(f"Subfolders: {subfolders}")
        
        folders = ["assets", "__MACOSX", "__pycache__"]
        for folder in folders:
            if folder in subfolders:
                subfolders.remove(folder)
                self.logger.info(f"Removed {folder} folders")
            
        self.logger.info(f"Subfolders: {subfolders}")
        
        if len(subfolders) < 2:
            raise ValueError(f"input zip file does not contain the necessary directories, {self.training_file_id}")

        # try:
        #     os.remove(zip_file)
        #     self.logger.info(f"Removed Zip File: {zip_file}")
        # except Exception as e:
        #     self.logger.info(f"No Zip file found to be removed: {e}")

        self.base_path = f"{training_model_dir}/{subfolders[0]}/{subfolders[1]}"
        algo_name = subfolders[0]
        self.logger.info(f'base path: {self.base_path}')

        path_to_training_data = f"{self.base_path}/data"
        self.logger.info(f'data file dir path: {path_to_training_data}')
        
        # Use glob to find all CSV files in the folder
        csv_file_path = glob.glob(os.path.join(f"{path_to_training_data}/", '*.csv'))[0]
        
        config_json_path = glob.glob(os.path.join(f"{path_to_training_data}/", '*.json'))[0]

        # Extract the file names from the paths
        full_csv_file_path = path_to_training_data + "/" + os.path.basename(csv_file_path)
        self.logger.info(f'Fully Qualified csv Path: {full_csv_file_path}')
        
        full_config_json_path = path_to_training_data + "/" + os.path.basename(config_json_path)
        self.logger.info(f'Fully Qualified config json Path: {full_config_json_path}')

        return DataSourceInfo(csv_file_path=full_csv_file_path, 
                            algo_name=algo_name, 
                            config_file_path=full_config_json_path)

    @abstractmethod
    def update_status(self, success: bool, message: str) -> str:
        pass
