import json
import os
import time
from enum import Enum
from typing import Optional
from pydantic import BaseModel


class Status(str, Enum):
    SUCCESS = "SUCCESS"
    FAILURE = "FAILURE"
    INPROGRESS = "INPROGRESS"
    STARTED = "STARTED"

class HedgePipelineStatus(BaseModel):
    status: Status = Status.STARTED
    message: str = ""
    timestamp: str = ""
    file_path: str = "status.json"
    
    def update(self, new_status: Optional[Status] = None, new_message: Optional[str] = None):
        if new_status:
            self.status = new_status
        if new_message:
            self.message += f"{new_message}\n"
        self.timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        
        # Load existing logs if available
        logs = []
        if os.path.exists(self.file_path):
            try:
                with open(self.file_path, 'r') as f:
                    logs = json.load(f).get("logs", [])
            except json.JSONDecodeError as je:
                print(f'JSON Exception while opeing status json file:{je}')
                pass

        logs.append({"timestamp": self.timestamp, "message": self.message})
        update = {"status": self.status, "last_updated": self.timestamp, "logs": logs}

        print(f'Updating status->{update}')
        # Atomically write the update to the file
        temp_path = f"{self.file_path}.tmp"
        with open(temp_path, 'w') as f:
            json.dump(update, f, indent=2)
        os.replace(temp_path, self.file_path)
        
    @property
    def is_success(self):
        return self.status == Status.SUCCESS

    def __str__(self):
        return f"Status: {self.status.value}, Message: {self.message}"
