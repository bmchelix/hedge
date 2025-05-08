from fastapi import Request, status
from fastapi.responses import JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.exceptions import RequestValidationError
from fastapi import FastAPI, HTTPException, Depends, APIRouter, Path
from common.src.util.logger_util import LoggerUtil
from common.src.ml.hedge_inference import HedgeInferenceBase, Inputs, Outputs
from common.src.util.infer_exception import HedgeInferenceException


# Parent Class
class InferenceAPI(FastAPI):
    def __init__(self, inference_obj, inputs: Inputs, outputs: Outputs, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self.logger = LoggerUtil().logger

        # Store the service instance
        self.inference_obj = inference_obj

        self.inputs = inputs

        # Add CORS middleware
        self.add_middleware(
            CORSMiddleware,
            allow_origins=["*"],
            allow_credentials=True,
            allow_methods=["*"],
            allow_headers=["*"]
        )

        @self.exception_handler(HedgeInferenceException)
        async def http_inference_exception_handler(self, request: Request, exc: HedgeInferenceException):
            return JSONResponse(
                status_code=exc.status_code,
                content={
                    "detail": exc.detail,
                    "error_code": exc.error_code,
                    "path": str(request.url.path),
                },
                headers=exc.headers,  # Optional: Include custom headers if any
            )

        @self.exception_handler(RequestValidationError)
        async def validation_exception_handler(request: Request, exc: RequestValidationError):
            exc_str = f'{exc}'.replace('\n', ' ').replace('   ', ' ')
            self.logger.error(f"{request}: {exc_str}")
            self.logger.info(f"{request._body.decode('utf-8')}")

            content = {'status_code': 10422, 'message': exc_str, 'data': None}
            return JSONResponse(content=content, status_code=status.HTTP_422_UNPROCESSABLE_ENTITY)

        # Add common routes to the app directly
        @self.get("/")
        def homepage():
            return {"message": "Welcome to the Hedge Inference API!"}

        # Access the parent router indirectly by defining new routers in the child
        router = APIRouter(prefix="/api/v3", tags=["Inferencing"])

        @router.post("/reinitialize")
        def reinitialize(inference_obj: HedgeInferenceBase= Depends(self.get_service)):
            try:
                inference_obj.load_model(inference_obj.model_dir, inference_obj.artifacts_dict)
                return {"message": "Reinitialization successful"}
            except Exception as e:
                raise HTTPException(status_code=500, detail=f"Reinitialization failed: {e}")

        @router.post("/predict/{ml_algorithm}/{training_config}")
        async def get_prediction(ml_algorithm: str = "HedgeAnomaly", training_config: str = Path(),
                                inputs:Inputs = self.inputs):
            try:
                self.logger.info(f'training config: {training_config}, ml algorithm: {ml_algorithm}')
                self.logger.info(f'{inputs=}')
                final_predictions = inference_obj.predict(ml_algorithm, training_config, inputs)
                self.logger.info(f'{final_predictions=}')
                return final_predictions
            except Exception as e:
                inference_obj.logger.error(f"Prediction failed: {e}")
                raise HTTPException(status_code=500, detail=f"Prediction failed: {e}")

        # Include the child router in the app
        self.include_router(router)

    # Method to return the service instance
    def get_service(self):
        return self.inference_obj
