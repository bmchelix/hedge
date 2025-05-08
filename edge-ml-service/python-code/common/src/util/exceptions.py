class HedgeTrainingException(Exception):
    """
    Custom exception for training pipeline with error codes.
    """
    def __init__(self, message, error_code=None):
        """

        Args:
            message (_type_): _description_
            error_code (_type_, optional): _description_. Defaults to None.
        """
        super().__init__(message)
        self.message = message
        self.error_code = error_code

    def __str__(self):
        return f"[Error {self.error_code}] {self.message}" if self.error_code else self.message

