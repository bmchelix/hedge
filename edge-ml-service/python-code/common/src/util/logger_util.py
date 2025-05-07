import logging
from logging import Logger


class LoggerUtil:
    logger: Logger

    class CustomFormatter(logging.Formatter):
        blue = "\x1b[34;20m"
        green = "\x1b[32;20m"
        yellow = "\x1b[33;20m"
        red = "\x1b[31;20m"
        bold_red = "\x1b[31;1m"
        reset = "\x1b[0m"
        format_string = (
            "%(asctime)s - %(levelname)s - %(message)s (%(filename)s:%(lineno)d)"
        )

        FORMATS = {
            logging.DEBUG: blue + format_string + reset,
            logging.INFO: green + format_string + reset,
            logging.WARNING: yellow + format_string + reset,
            logging.ERROR: red + format_string + reset,
            logging.CRITICAL: bold_red + format_string + reset,
        }

        def format(self, record):
            log_fmt = self.FORMATS.get(record.levelno)
            formatter = logging.Formatter(log_fmt)
            return formatter.format(record)

    def __init__(self, log_level=logging.DEBUG):
        self.log_level = log_level
        self.logger: Logger = self.configure_logging()

    def configure_logging(self) -> Logger:
        """Configure logging with formatters and stream handlers"""

        logger = logging.getLogger(__name__)
        if logger.hasHandlers():
            logger.handlers.clear()

        ch = logging.StreamHandler()
        ch.setLevel(self.log_level)
        ch.setFormatter(self.CustomFormatter())
        logger.addHandler(ch)
        logger.setLevel(self.log_level)

        return logger
