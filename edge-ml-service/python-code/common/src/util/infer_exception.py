from fastapi import HTTPException

class HedgeInferenceException(HTTPException):
    """
    Custom HTTP exception for inference-related errors, extending HTTPException.
    """

    def __init__(self, status_code: int, detail: str, error_code: int = None, headers: dict = None):
        """
        Initialize the HttpInferenceException.

        Args:
            status_code (int): HTTP status code for the response.
            detail (str): Detailed error message.
            error_code (int, optional): Custom application-specific error code.
            headers (dict, optional): Additional headers for the HTTP response.
        """
        super().__init__(status_code=status_code, detail=detail, headers=headers)
        self.error_code = error_code  # Custom application-specific error code

    def __str__(self):
        return f"[HTTP {self.status_code}] {self.detail} (Error Code: {self.error_code})"