"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
from __future__ import absolute_import
from __future__ import division
from __future__ import print_function
import traceback
import joblib
import shutil
import os
import numpy as np
import pandas as pd
from sklearn.mixture import GaussianMixture
from sklearn.model_selection import train_test_split

from common.src.ml.hedge_status import Status
from common.src.ml.hedge_training import HedgeTrainingBase
from common.src.util.logger_util import LoggerUtil
from common.src.util.env_util import EnvironmentUtil
from common.src.util.data_transformer import DataFramePreprocessor


class GMMClustering(HedgeTrainingBase):
    def __init__(self):
        self.logger_util = LoggerUtil()
        self.logger = self.logger_util.logger

        # Pass this as environment variable (local, job_dir)
        self.local = False
        self.data_util = None

        env_file: str = os.path.join(os.getcwd(), "clustering", "gaussian_mixture_model", "env.yaml")
        self.env_util = EnvironmentUtil(env_file, args=['TRAINING_FILE_ID'])

        self.base_path = ''
        self.algo_name = ''
        self.artifact_dictionary = {}

        self.n_components = 1
        self.covariance_type = 'full'
        self.max_iter = 100
        self.random_state = 42
        self.verbose = 1

        self.gmm_model = None
        self.scaler = None


    def save_artifacts(self, artifact_dictionary, model):
        export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
        os.makedirs(export_path, exist_ok=True)

        try:
            self.logger.info(f"Attempting to save artifacts at: {export_path}")
            self.logger.info(f"artifact_dictionary: {artifact_dictionary}")
            self.logger.info(f"model: {model}")

            joblib.dump(artifact_dictionary, f'{export_path}/artifacts.gz', compress=True)
            self.logger.info(f"Artifact dictionary saved successfully at: {export_path}/artifacts.gz")

            joblib.dump(model, f'{export_path}/model.gz', compress=True)
            self.logger.info(f"Model saved successfully at: {export_path}/model.gz")

        except Exception as e:
            self.logger.error(f"Failed to save model and artifacts: {e}")

        os.chdir(self.job_dir)
        model_zip_file = shutil.make_archive('hedge_export', 'zip', root_dir=".", base_dir=f'./{self.algo_name}')
        self.data_util.upload_data(model_zip_file)

    def execute_training_pipeline(self):
        try:
            self.logger.info("Getting Environment Variables")
            result = self.get_env_vars()
            if result == self.FAILURE:
                self.logger.warning("Failed to get environment variables")

            self.logger.info("Training GMM Clustering Model...")
            self.train()

            self.logger.info("Generating Training Summary...")
            self.create_and_save_summary()

            self.logger.info("Exporting Model...")
            self.save_artifacts(self.artifact_dictionary, self.gmm_model)
            self.pipeline_status.update(Status.SUCCESS, "End of Training")
            self.data_util.update_status(True, "")
        except Exception as e:
            self.logger.error(f"Training failed: {e}")
            self.logger.error(traceback.print_exc())
            self.data_util.update_status(False, str(e))

    def get_env_vars(self) -> str:

        super().get_env_vars()
        self.max_iter = self.env_util.get_env_value("MAX_ITER", self.max_iter)
        return HedgeTrainingBase.SUCCESS


    def train(self):
        self.logger.info("Getting Model Variables & Configurations")

        data_source_info = self.data_util.read_data()
        self.algo_name = data_source_info.algo_name
        full_csv_file_path = data_source_info.csv_file_path

        self.pipeline_status.update(Status.INPROGRESS, "Start of Training")
        df = pd.read_csv(full_csv_file_path)
        df = df.fillna(0.0)

        preprocessor = DataFramePreprocessor()
        df_train, df_validate = train_test_split(df, test_size=0.2, random_state=self.random_state)
        df_train_transformed = preprocessor.fit_transform(df_train)

        n_components_range = range(1, 11)
        best_gmm_model = None
        best_bic_score = np.inf
        best_n_components = 1

        self.logger.info(f"Starting GMM model selection over {len(n_components_range)} components...")


        for n in n_components_range:
            self.logger.info(f"Training GMM with {n} components...")

            gmm = GaussianMixture(
                n_components=n,
                covariance_type=self.covariance_type,
                max_iter=self.max_iter,
                random_state=self.random_state
            )

            gmm.fit(df_train_transformed)


            bic_score = gmm.bic(df_train_transformed)
            self.logger.info(f"BIC score for {n} components: {bic_score}")

            if bic_score < best_bic_score:
                best_bic_score = bic_score
                best_gmm_model = gmm
                best_n_components = n


        self.logger.info(f"Best number of components: {best_n_components} with BIC score: {best_bic_score}")
        self.n_components = best_n_components

        self.gmm_model = best_gmm_model

        self.artifact_dictionary = {
            'transformer': preprocessor.get_preprocessor(),
            'scaler': preprocessor.get_standard_scaler(),
            'n_components': best_n_components
        }

    def create_and_save_summary(self):
        path_to_training_data = f"{self.data_util.base_path}/data"
        export_path = os.path.join(f"{self.data_util.base_path}", "hedge_export")
        self.logger.info(f"export path for local model file: {export_path}")
        os.makedirs(export_path, exist_ok=True)

        asset_path = os.path.join(f"{export_path}", "assets")
        os.makedirs(asset_path, exist_ok=True)
        config_file_path = f'{path_to_training_data}/config.json'

        shutil.copy2(config_file_path, asset_path)

        summary_text = (
            "Model Statistics:\n"
            f"Number of Components (Clusters): {self.n_components}\n"
            f"Covariance Type: {self.covariance_type}\n"
            f"Max Iterations: {self.max_iter}\n"
            "\n"
            "Training Statistics:\n"
            f"GMM Model Parameters: {self.gmm_model.get_params()}\n"
        )

        try:
            with open(f'{asset_path}/training_summary.txt', "w") as model_summary:
                model_summary.write(summary_text)
            self.logger.info(f"Training summary saved to {asset_path}/training_summary.txt")
        except Exception as e:
            self.logger.warning(f"Unable to save training summary to {asset_path}/training_summary.txt: {e}")

        return summary_text


if __name__ == "__main__":
    GMMClustering().execute_training_pipeline()
