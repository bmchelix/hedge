
import watchdog
from watchdog.events import FileSystemEvent, FileSystemEventHandler
import watchdog.events
from watchdog.observers import Observer
from .hedge_inference import HedgeInferenceBase
from threading import Lock
from common.src.util.logger_util import LoggerUtil


watcher_lock: Lock = Lock()
reload: bool = False

logger = LoggerUtil().logger

class ModelWatcher(FileSystemEventHandler):
    def __init__(self, inference_engine: HedgeInferenceBase):
        self.inference_engine = inference_engine

    def on_any_event(self, event: FileSystemEvent) -> None:
        logger.info(f"event: {event}, reloading artifacts from {self.inference_engine.model_dir}")
        if event.event_type == watchdog.events.EVENT_TYPE_MODIFIED:
            watcher_lock.acquire()
            self.inference_engine.load_model(
                self.inference_engine.model_dir, self.inference_engine.artifacts_dict
            )
            watcher_lock.release()


def start_fs_monitor(engine: HedgeInferenceBase) -> None:
    event_handler = ModelWatcher(inference_engine=engine)
    observer = Observer()
    logger.info(f"model dir: {engine.model_dir}")
    observer.schedule(event_handler=event_handler, path=engine.model_dir, recursive=True)
    observer.start()
    logger.info("started start_fs_monitor")
