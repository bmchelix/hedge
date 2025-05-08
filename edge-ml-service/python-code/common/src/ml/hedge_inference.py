"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import os
import traceback
import importlib
import io
import zipfile
import zipimport
import logging
from logging import Logger
from abc import ABC, abstractmethod
from typing import Any, Optional, List, Tuple, Dict
from pydantic import BaseModel
from common.src.util.logger_util import LoggerUtil


# Base Input model that can be extended
class Inputs(BaseModel):
    __root__: Optional[Dict[str, Any]]

class Outputs(BaseModel):
    __root__: Dict[str, Any]


class HedgeInferenceBase(ABC):
    logger: Logger = logging.getLogger()
    model_dict: Optional[Dict[str, Any]]

    @abstractmethod
    def __init__(self):
        self.logger = LoggerUtil().logger
        pass

    @abstractmethod
    def read_model_config(self):
        pass

    # @abstractmethod
    # def load_model(self) -> str:
    #     pass

    @abstractmethod
    def predict(self, ml_algorithm: str, training_config: str, external_input: Inputs) -> Any:
        pass

    def _get_env(self, key: str, app_settings: str, default_value: str) -> str:
        """
        Wrapper function for self.env_util.get_env_value(). Takes a key, app settings and a default
        Searches for the value in this order -- environment variable set, app settings and then default
        """
        return self.env_util.get_env_value(key, self.env_util.get_env_value(app_settings, default_value))

    def get_env_vars(self):
        """
        Read environment variables and set common training parameters
        """

        self.local = self._get_env(
            "LOCAL", "ApplicationSettings.Local", "/"
        )
        self.model_dir = self._get_env(
            "MODELDIR", "ApplicationSettings.ModelDir", "/res/edge/models",
        )
        self.output_dir = self._get_env(
            "OUTPUTDIR", "ApplicationSettings.OutputDir", "/res/edge/models",
        )
        self.port = self._get_env("PORT", 'Service.port', '48096')
        self.host = self._get_env("HOST", 'Service.host', '0.0.0.0')

        self.logger.info(f"LOCAL: {self.local}")
        self.logger.info(f"output_dir: {self.output_dir}")
        self.logger.info(f"MODELDIR: {self.model_dir}")
        self.logger.info(f"PORT: {self.port}")
        self.logger.info(f"HOST: {self.host}")

        # Initialize Local or Cloud Training


    def __load_artifact(self, file_path) -> Any:
        """Load Artifact file from the given file path

        Args:
            file_path (model and other artifact extensions): Model and other artifact files

        Raises:
            FileNotFoundError: File not found

        Returns:
            _type_: Loaded Object file
        """

        obj: Any = None
        if not os.path.exists(file_path):
            raise FileNotFoundError(f"The specified path does not exist: {file_path}")
        try:
            if file_path.endswith('.h5') or file_path.endswith('.hdf5'):
                tf_keras = importlib.import_module('tensorflow.keras.models')
                self.logger.info("Loading object with tensorflow libs...")
                with open(file_path, 'rb') as serialized_file:
                    obj = tf_keras.load_model(serialized_file)
                self.logger.info("Object loaded successfully with tensorflow.")
            elif file_path.endswith('.json'):
                json_module = importlib.import_module('json')
                self.logger.info("Loading object with json...")
                with open(file_path, 'r') as serialized_file:
                    obj = json_module.load(serialized_file)
                self.logger.info("Object loaded successfully with json")
            else:
                joblib = importlib.import_module('joblib')
                self.logger.info("Loading object with joblib...")
                with open(file_path, 'rb') as serialized_file:
                    obj = joblib.load(serialized_file)
                self.logger.info("Object loaded successfully with joblib.")
        except Exception as e:
                traceback.print_exc()
                self.logger.warning(f"Not able to load artifact file {file_path}, " +
                                    f"failed validation: {str(e)}")
        return obj


    def __load_zipped_artifact(self, file_path: str, importer: zipimport.zipimporter) -> Any:
        """Load Artifact file from the given file path

        Args:
            file_path (model and other artifact extensions): Model and other artifact files

        Raises:
            OSError: when file is not found

        Returns:
            _type_: Loaded Object file
        """
        data: bytes = importer.get_data(file_path)
        with io.BytesIO(data) as serialized_file:
            try:
                if file_path.endswith('.h5') or file_path.endswith('.hdf5'):
                    tf_keras = zipimport.import_module('tensorflow.keras.models')
                    self.logger.info("Loading object with tensorflow libs...")
                    obj = tf_keras.load_model(serialized_file)
                    self.logger.info("Object loaded successfully with tensorflow.")
                elif file_path.endswith('.json'):
                    json_module = importlib.import_module('json')
                    self.logger.info("Loading object with json...")
                    obj = json_module.load(serialized_file)
                    self.logger.info("Object loaded successfully with json")
                else:
                    joblib = importlib.import_module('joblib')
                    self.logger.info("Loading object with joblib...")
                    obj = joblib.load(serialized_file)
                    self.logger.info("Object loaded successfully with joblib.")
            except Exception as e:
                    traceback.print_exc()
                    self.logger.warning(f"Not able to load artifact file {file_path}, " +
                                        f"failed validation: {str(e)}")
        return obj


    def clear_and_initialize_model_dict(self, keys):
        # Initialize the model dictionary
        model_dict = {}
        for key in keys:
            model_dict[key] = {}
        return model_dict


    def load_model(self, base_path: str, artifacts_dict: dict) -> dict:
        """Initialize and load all models, encoders, and scalers."""
        self.logger.info("Initialization In Process")

        # Clear the existing model_dict if it exists
        try:
            self.model_dict = self.clear_and_initialize_model_dict(artifacts_dict.keys())
        except Exception as e:
            self.logger.warning(f"Could not initialize model dict: {str(e)}")

        self._check_and_create_folder(base_path)
        self.logger.debug(f'Model Base Path: {base_path}')

        ml_algorithms: List[str] = os.listdir(base_path)
        # Load all models, encoders, and scalers for each algorithm
        for ml_algorithm in ml_algorithms:

            self.logger.debug(f"ML Algorithm: {ml_algorithm}")

            file_name: str =  f"{base_path}/{ml_algorithm}"
            is_file: bool = os.path.isfile(file_name)
            zipped_data: bool = self._is_zip_file(file_name)
            if is_file:
                if zipped_data:
                    self._load_artifacts_from_zip_file(file_name, artifacts_dict)
                continue

            model_cfgs: Optional[List[str]] = []
            model_cfgs, ok = self._get_model_cfgs(ml_algorithm, base_path)
            if not ok:
                continue


            # initialize model data
            self._init_model_dict(ml_algorithm, artifacts_dict)

            self.logger.info(f"model cfgs: {model_cfgs}")
            ## Load each model configuration corresponding to this algorithm
            for model_cfg in model_cfgs:
                ok: bool = self._load_model_artifacts(base_path, ml_algorithm, model_cfg, artifacts_dict)
                if ok:
                    self.logger.info(f"SUCCESS: ML Algorithm: {ml_algorithm} and Model: {model_cfg} were loaded")
                else:
                    self.logger.error(f"FAILURE loading ML Algorithm {ml_algorithm} and ML Cfg {model_cfg}")

        self.logger.info(f"Model Dict: {self.model_dict}")
        return self.model_dict


    def _load_model_artifacts(self, base_path: str, ml_algorithm: str, model_cfg: str, artifacts_dict: Dict[str, Any]) -> bool:
        """
        Loads Model artifacts like model file, config.json etc.
        """
        ok: bool = True

        try:
            model_path = os.path.join(os.getcwd(), base_path, ml_algorithm, model_cfg, "hedge_export")
            self.logger.info(f"model cfg path: {model_path}")

            model_ignore_file = os.path.join(model_path, ".modelignore")
            if os.path.exists(model_ignore_file):
                self.logger.warning(f"Not loading the model config: {model_cfg}")

            self.logger.info(f"artifacts : {artifacts_dict}")
            for file_type in artifacts_dict:
                artifact_path = os.path.join(model_path, artifacts_dict[file_type])
                self.logger.info(f"Attempting to load artifact file: {artifact_path}")

                if not os.path.exists(artifact_path):
                    self.logger.error(f"{artifact_path} does not exist")
                    ok = False
                    break

                self.model_dict[file_type][ml_algorithm][model_cfg] = self.__load_artifact(artifact_path)
        except Exception as e:
                self.logger.warning("Error reading the model and config files, " +
                                        f"this model will not be loaded because {e}")
                ok = False

        return ok


    def _check_and_create_folder(self, path: str) -> None:
        """
        Check if the folder exists and create it if not.
        """

        if not os.path.exists(path):
            try:
                self.logger.warning(f"Creating directory at {path}")
                os.makedirs(path, mode=0o775, exist_ok=True)
                self.logger.warning(f"Successfully created directory at {path}")
            except Exception as e:
                self.logger.warning(f"Could not create directory at {path}: f{str(e)}")


    def _get_model_cfgs(self, ml_algorithm: str, base_path: str) -> Tuple[List[str], bool]:
        """
        Get the model configuration names as the names of subfolders in the directory for ml_algorithm
        """

        ok: bool = True
        model_cfgs: List[str] = []
        try:
            model_cfgs = os.listdir(f"{base_path}/{ml_algorithm}")
            self.logger.info(f'{model_cfgs=}')
            if '.DS_Store' in model_cfgs:
                model_cfgs.remove('.DS_Store')
        except Exception as e:
            self.logger.warning(f"Could not list directory at {base_path}/{ml_algorithm}, ignoring: {e}")
            ok = False

        return model_cfgs, ok


    def _init_model_dict(self, ml_algorithm: str, artifacts_dict: Dict[str, Any]) -> None:
        """
        Initialize the model dict
        """
        if self.model_dict is None:
            return

        for file_type in artifacts_dict:
            self.model_dict[file_type][ml_algorithm] = {}


    def _is_zip_file(self, file_name: str) -> bool:
        """
        Check  whether the file is a zip file
        """
        return zipfile.is_zipfile(file_name)


    def _load_artifacts_from_zip_file(self, file_name, artifacts_dict: Dict[str, Any]) -> bool:
        """
        Load artifacts from hedge_export.zip
        """

        if self.model_dict is None:
            return False

        try:

            importer: zipimport.zipimporter = zipimport.zipimporter(file_name)

            with  zipfile.ZipFile(file_name, 'r') as file:
                file_list: List[str] = file.namelist()
                ml_algorithm: str = file_list[0].replace("/", "",1)
                ml_cfg: str = file_list[1].split("/")[1]

                self.logger.info(f"ML Algorithm: {ml_algorithm}, ML Config: {ml_cfg}")
                self._init_model_dict(ml_algorithm, artifacts_dict)

            for file in file_list[2:]:
                base_name:str = os.path.basename(file)
                if not base_name:
                    continue

                dir_name: str = os.path.dirname(file).replace("/", "",1)

                for i, (file_type, name) in enumerate(artifacts_dict.items()):

                    self.logger.debug(f"{file_type}:{name}, {dir_name}/{base_name}")

                    # Make sure the file isn't loaded already
                    not_loaded: bool = self.model_dict.get(file_type, {}).get(ml_algorithm, {}).get(ml_cfg, None) is None

                    # Ensure that we are loading assets/config.json
                    correct_assets_file: bool = (base_name == "config.json" and "assets" in dir_name)

                    # Make sure the name of the file matches
                    file_name_matches: bool = base_name in name

                    if file_name_matches and  (correct_assets_file or not_loaded):
                            self.model_dict[file_type][ml_algorithm][ml_cfg] = self.__load_zipped_artifact(file, importer)


        except Exception as e:
            self.logger.error(f"Exception reading zipfile: {file_name}: {str(e)}")
            return False

        return True
