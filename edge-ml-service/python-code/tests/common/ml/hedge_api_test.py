"""
(c) Copyright 2020-2025 BMC Software, Inc.

Contributors: BMC Software, Inc. - BMC Helix Edge
"""
import unittest
import asyncio
import json
from unittest.mock import MagicMock, patch
from fastapi.testclient import TestClient
from fastapi import HTTPException, status, Request
from fastapi.exceptions import RequestValidationError

from starlette.requests import Request
from starlette.responses import JSONResponse

from common.src.ml.hedge_api import InferenceAPI
from common.src.util.infer_exception import HedgeInferenceException

class TestHedgeApi(unittest.TestCase):
    def setUp(self):
        self.mock_inference_util = MagicMock()
        self.mock_inputs = MagicMock()
        self.mock_outputs = MagicMock()

        self.app = InferenceAPI(
            inference_obj=self.mock_inference_util,
            inputs=self.mock_inputs,
            outputs=self.mock_outputs
        )

        self.client = TestClient(self.app)

    def test_homepage(self):
        response = self.client.get("/")
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.assertEqual(response.json(), {"message": "Welcome to the Hedge Inference API!"})

    def test_reinitialize_success(self):
        self.mock_inference_util.load_model.return_value = None

        response = self.client.post("/api/v3/reinitialize")
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.assertEqual(response.json(), {"message": "Reinitialization successful"})
        self.mock_inference_util.load_model.assert_called_once()

    def test_reinitialize_failure(self):
        self.mock_inference_util.load_model.side_effect = Exception("Test error")

        response = self.client.post("/api/v3/reinitialize")
        self.assertEqual(response.status_code, status.HTTP_500_INTERNAL_SERVER_ERROR)
        self.assertIn("Reinitialization failed: Test error", response.text)

    def test_predict_success(self):
        self.mock_inference_util.predict.return_value = {"result": "success"}

        ml_algorithm = "HedgeAnomaly"
        training_config = "test_config"

        response = self.client.post(f"/api/v3/predict/{ml_algorithm}/{training_config}")
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.assertEqual(response.json(), {"result": "success"})

        self.mock_inference_util.predict.assert_called_once_with(
            ml_algorithm,
            training_config,
            self.mock_inputs
        )

    def test_predict_failure(self):
        self.mock_inference_util.predict.side_effect = Exception("Prediction failed")

        ml_algorithm = "HedgeAnomaly"
        training_config = "test_config"

        response = self.client.post(f"/api/v3/predict/{ml_algorithm}/{training_config}")
        self.assertEqual(response.status_code, status.HTTP_500_INTERNAL_SERVER_ERROR)
        self.assertIn("Prediction failed: Prediction failed", response.text)

    def test_validation_error_handling(self):
        """
        Since the route code catches *all* exceptions and re-raises as HTTP 500,
        we expect a 500. That is consistent with the actual route logic.
        """
        from fastapi.exceptions import RequestValidationError

        self.mock_inference_util.predict.side_effect = RequestValidationError([], body=None)

        response = self.client.post("/api/v3/predict/HedgeAnomaly/test_config", json={})
        self.assertEqual(response.status_code, status.HTTP_500_INTERNAL_SERVER_ERROR)
        self.assertIn("Prediction failed", response.text)

    def test_custom_inference_exception_handling(self):
        self.mock_inference_util.predict.side_effect = HedgeInferenceException(
            detail="Custom inference error",
            status_code=400,
            error_code="HEDGE_ERROR"
        )

        response = self.client.post("/api/v3/predict/HedgeAnomaly/test_config")

        self.assertEqual(response.status_code, 500)

        self.assertIn("Custom inference error", response.text)
        self.assertIn("HEDGE_ERROR", response.text)

    def test_hedge_inference_exception_handler_coverage(self):
        handler = self.app.exception_handlers.get(HedgeInferenceException)
        self.assertIsNotNone(handler, "No HedgeInferenceException handler found in the app")

        scope = {
            "type": "http",
            "method": "GET",
            "path": "/api/v3/predict/HedgeAnomaly/test_config",
            "raw_path": b"/api/v3/predict/HedgeAnomaly/test_config",
            "query_string": b"",
            "headers": [],
        }
        mock_request = Request(scope)

        exc = HedgeInferenceException(
            detail="Custom inference error detail",
            status_code=418,
            error_code="HEDGE_ERROR_CODE",
            headers={"X-Test-Header": "HeaderValue"}
        )

        response = asyncio.run(handler(self.app, mock_request, exc))

        self.assertIsInstance(response, JSONResponse)
        self.assertEqual(response.status_code, 418)

        body_bytes = response.body
        content = json.loads(body_bytes.decode("utf-8"))

        self.assertEqual(content["detail"], "Custom inference error detail")
        self.assertEqual(content["error_code"], "HEDGE_ERROR_CODE")
        self.assertEqual(content["path"], "/api/v3/predict/HedgeAnomaly/test_config")

        self.assertIn("X-Test-Header", response.headers)
        self.assertEqual(response.headers["X-Test-Header"], "HeaderValue")


if __name__ == '__main__':
    unittest.main()
