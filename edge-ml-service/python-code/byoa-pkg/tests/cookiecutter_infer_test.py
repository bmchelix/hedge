import unittest
import os
import tempfile
import shutil
import sys
import importlib.util
import subprocess
import time
import requests  # pip install requests if you don't have it
from cookiecutter.main import cookiecutter


class TestCookiecutterInference(unittest.TestCase):
    def setUp(self):
        self.original_cwd = os.getcwd()
        self.temp_dir = tempfile.mkdtemp()

    def tearDown(self):
        os.chdir(self.original_cwd)
        shutil.rmtree(self.temp_dir)

    def test_infer_run_and_confirm_server(self):
        this_file_dir = os.path.dirname(os.path.abspath(__file__))
        template_dir = os.path.abspath(os.path.join(this_file_dir, ".."))

        cookie_json_path = os.path.join(template_dir, "cookiecutter.json")
        self.assertTrue(os.path.isfile(cookie_json_path), f"No cookiecutter.json found in: {template_dir}")

        # Cookiecutter overrides
        context_overrides = {
            "ml_algo": "TestAlgo",
            "ml_model": "TestModel",
            "common_libraries_folder": os.path.join(self.original_cwd, "common"),
            "algo_type": "Anomaly",
            "infer_class_name": "MyInferenceClass"
        }

        cookiecutter(
            template_dir,
            no_input=True,
            extra_context=context_overrides,
            output_dir=self.temp_dir
        )

        base_path = os.path.join(self.temp_dir, "TestAlgo", "TestModel")
        env_yaml_path = os.path.join(base_path, "env.yaml")
        self.assertTrue(os.path.exists(env_yaml_path), f"Expected env.yaml at {env_yaml_path}")

        with open(env_yaml_path, "w") as f:
            f.write(
                """ApplicationSettings:
  OutputDir: ./fake_models
  ModelDir: ./fake_models
  training_file_id: /tmp/training.zip
  Local: True
Service:
  host: '0.0.0.0'
  port: '55000'
"""
            )
        os.makedirs(os.path.join(base_path, "fake_models"), exist_ok=True)

        infer_script = os.path.join(base_path, "infer", "src", "task.py")
        self.assertTrue(
            os.path.exists(infer_script),
            f"Expected {infer_script} to exist but was not found."
        )

        script_dir = self.temp_dir
        env = os.environ.copy()

        process = subprocess.Popen(
            [sys.executable, infer_script],
            cwd=script_dir,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            env=env
        )

        # wait to allow uvicorn to start
        time.sleep(2)


        server_url = "http://0.0.0.0:55000/"

        try:
            response = requests.get(server_url, timeout=2)
            self.assertIn(response.status_code, [200, 404], f"Unexpected status code {response.status_code}")
        except requests.exceptions.RequestException as e:
            self.fail(f"Could not connect to the inference server: {e}")

        process.terminate()
        stdout, stderr = process.communicate(timeout=5)

        print("\n-- INFERENCE STDOUT --\n", stdout)
        print("\n-- INFERENCE STDERR --\n", stderr)


if __name__ == "__main__":
    unittest.main()
