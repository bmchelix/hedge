"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from __future__ import absolute_import, division, print_function
import os
import traceback
import importlib
import logging
from logging import Logger
from abc import ABC, abstractmethod
from typing import Any
from common.src.ml.hedge_status import HedgePipelineStatus, Status
from common.src.util.env_util import EnvironmentUtil
from typing import Optional



class HedgeTrainingBase(ABC):

    logger: Logger = logging.getLogger()
    pipeline_status: Optional[HedgePipelineStatus]
    env_util: Optional[EnvironmentUtil]

    @abstractmethod
    def save_artifacts(self, artifact_dictionary: Any, model: Any) -> str:
        pass

    @abstractmethod
    def train(self) -> str:
        pass

    def _initialize_data_util(self):
        """
        Initialize parameters used for training
        """
        if self.env_util is None:
            return

        try:
            if isinstance(self.local, str) and self.local.lower() == "true":
                local_util_module = importlib.import_module(
                    "common.src.data.impl.local_training_util"
                )
                self.data_util = getattr(local_util_module, "LocalTrainingUtil")()
                self.data_util.model_dir = self.model_dir
                self.data_util.training_file_id = self.training_file_id
            else:
                raise Exception("Cloud Training is not supported, Check yours environment file!")
        except Exception as e:
            self.logger.error(f"Error initializing data util: {str(e)}")
            self.pipeline_status.update(Status.FAILURE, f"Error initializing data util: {e}")

        self.logger.info(f"Initialized Util:: {self.data_util}")


    def _get_env(self, key: str, app_settings: str, default_value: str) -> str:
        """
        Wrapper function for self.env_util.get_env_value(). Takes a key, app settings and a default
        Searches for the value in this order -- environment variable set, app settings and then default
        """

        if self.env_util is None:
            return default_value

        return self.env_util.get_env_value(key, self.env_util.get_env_value(app_settings, default_value))

    def get_env_vars(self):
        """
        Read environment variables and set common training parameters
        """

        self.local = self._get_env(
            "LOCAL", "ApplicationSettings.Local", "/"
        )
        self.model_dir = self._get_env(
            "MODELDIR", "ApplicationSettings.ModelDir", "/tmp/hedge",
        )
        # Training file id is passed as a parameter, not via environment variable
        self.training_file_id = self._get_env(
            "TRAINING_FILE_ID", "ApplicationSettings.training_file_id", "training_data.csv"
        )

        self.logger.info(f"LOCAL: {self.local}")
        self.logger.info(f"MODELDIR: {self.model_dir}")
        
        self.pipeline_status: HedgePipelineStatus = HedgePipelineStatus(status=Status.STARTED,
                                                                        file_path=f'{self.model_dir}/status.json')
        #if self.training_file_id == "training_data.csv":
        #    raise ValueError("Training Zip file is not passed. Please provide a valid training zip file.")

        # Initialize Local or Cloud Training
        self.logger.info("Initalizing Data Util")
        self._initialize_data_util()

        # Get Config File
        self.data_util.base_path = self.env_util.get_env_value(
            "OUTPUTDIR",
            self.env_util.get_env_value("ApplicationSettings.OutputDir", "/"),
        )



    def load_json(self, file_path):
        """Load Artifact file from the given file path

        Args:
            file_path: artifact file path

        Raises:
            FileNotFoundError: File not found

        Returns:
            _type_: Loaded Object file
        """
        if not os.path.exists(file_path):
            raise FileNotFoundError(f"The specified path does not exist: {file_path}")
        obj = None
        try:
            if file_path.endswith('.json'):
                json_module = importlib.import_module('json')
                self.logger.info("Loading object with json...")
                with open(file_path, 'r') as serialized_file:
                    obj = json_module.load(serialized_file)
                self.logger.info("Object loaded successfully with json")
        except Exception as ex:
                traceback.print_exc()
                self.logger.warning(f"Not able to load artifact file {file_path}, "
                                    f"failed validation: {ex}")

        if obj is None:
            raise ValueError(f"No object could be loaded from {file_path}")

        return obj
