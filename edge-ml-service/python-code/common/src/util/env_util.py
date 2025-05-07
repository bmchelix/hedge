import os
import yaml
import argparse
from argparse import ArgumentParser
from typing import Any, Dict
import flatdict


class EnvironmentUtil:
    file_names = ()

    def __init__(self, *file_names, args=[]):
        self.file_names = file_names
        self.args = args
        self.config: Dict[str, Any] = {}
        self.__load_files_to_env()

    def __load_files_to_env(self) -> None:
        """
        Update environment variables with values set from command line and from config files
        """

        config = {}
        for file_name in self.file_names:
            config_vars = create_dict_from_yaml(file_name)
            config.update(config_vars)

        # Add environment variables to the config
        config.update(os.environ)

        # Override config with command-line arguments (if provided)
        parser: ArgumentParser = argparse.ArgumentParser(
            description="Override environment variables with command-line arguments."
        )
        for arg in self.args:
            parser.add_argument(f"--{arg}", type=str)
        try:
            # Parse the command-line arguments
            args, _ = parser.parse_known_args()
        except Exception:
            return None

        config.update({k: v for k, v in vars(args).items() if v is not None})

        # Set the merged configuration back into environment variables
        for key in config:
            os.environ[key] = str(config[key])

        self.config = config

    def get_env_value(self, env_variable: str, default_value: str = "") -> str:
        return os.getenv(env_variable, default_value)


def create_dict_from_yaml(file_name) -> Dict[str, Any]:
    with open(file_name) as f:
        config_vars = yaml.safe_load(f)
    result: Dict[str, Any] = flatdict.FlatDict(config_vars, ".")
    return result
