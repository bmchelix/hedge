from common.src.data.hedge_util import HedgeUtil
from common.src.util.logger_util import LoggerUtil
from logging import Logger

class LocalTrainingUtil(HedgeUtil):
    
    logger: Logger
    model_dir: str
    def __init__(self):
        self.logger = LoggerUtil().logger
        pass

    def download_data(self) -> str:
        return self.training_file_id
    
    def upload_data(self, model_zip_file):
        pass
    
    def set_model_dir(self, model_dir, training_file_id) -> str:
        return model_dir

    def update_status(self, success: bool, message: str) -> str:
        status = "SUCCESSFUL" if success else "FAILED"
        self.logger.warning(f"Reporting status: {status}")
        if not success:
            self.logger.error(f"{message}")

        return status
