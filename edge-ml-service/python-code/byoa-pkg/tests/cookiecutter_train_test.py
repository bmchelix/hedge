import unittest
import os
import tempfile
import shutil
import sys
import importlib.util
import subprocess
from cookiecutter.main import cookiecutter


class TestCookiecutterTemplate(unittest.TestCase):

    def setUp(self):
        self.original_cwd = os.getcwd()
        self.temp_dir = tempfile.mkdtemp()

    def tearDown(self):
        os.chdir(self.original_cwd)
        shutil.rmtree(self.temp_dir)

    def test_render_and_basic_import(self):
        this_file_dir = os.path.dirname(os.path.abspath(__file__))
        template_dir = os.path.abspath(os.path.join(this_file_dir, ".."))

        cookie_json_path = os.path.join(template_dir, "cookiecutter.json")
        self.assertTrue(os.path.isfile(cookie_json_path), f"No cookiecutter.json found in: {template_dir}")

        # placeholder overrides
        context_overrides = {
            "ml_algo": "TestAlgo",
            "ml_model": "TestModel",
            "common_libraries_folder": os.path.join(self.original_cwd, "common"),
            "algo_type": "Anomaly",
            "training_class_name": "MyTrainingClass"
        }

        cookiecutter(
            template_dir,
            no_input=True,
            extra_context=context_overrides,
            output_dir=self.temp_dir
        )

        # debugging
        print("\nGenerated project structure:\n")
        for root, dirs, files in os.walk(self.temp_dir):
            level = root.replace(self.temp_dir, "").count(os.sep)
            indent = " " * (4 * level)
            print(f"{indent}{os.path.basename(root)}/")
            for f in files:
                print(f"{indent}    {f}")

        os.chdir(self.temp_dir)

        relative_task_path = os.path.join("TestAlgo", "TestModel", "train", "src", "task.py")
        absolute_task_path = os.path.abspath(relative_task_path)

        self.assertTrue(
            os.path.exists(absolute_task_path),
            f"Expected {absolute_task_path} to exist but it was not found."
        )

        spec = importlib.util.spec_from_file_location("task", absolute_task_path)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)

        MyTrainingClass = getattr(mod, "MyTrainingClass", None)
        self.assertIsNotNone(MyTrainingClass, "Could not find MyTrainingClass in task.py")

        instance = MyTrainingClass()
        self.assertIsNotNone(instance, "Failed to instantiate MyTrainingClass instance.")

    def test_rendered_structure(self):
        this_file_dir = os.path.dirname(os.path.abspath(__file__))
        template_dir = os.path.abspath(os.path.join(this_file_dir, ".."))

        cookie_json_path = os.path.join(template_dir, "cookiecutter.json")
        self.assertTrue(os.path.isfile(cookie_json_path), f"No cookiecutter.json found in: {template_dir}")

        context_overrides = {
            "ml_algo": "TestAlgo",
            "ml_model": "TestModel",
            "common_libraries_folder": os.path.join(self.original_cwd, "common"),
            "algo_type": "Anomaly",
            "training_class_name": "MyTrainingClass",
            "infer_class_name": "MyInferenceClass"
        }

        cookiecutter(
            template_dir,
            no_input=True,
            extra_context=context_overrides,
            output_dir=self.temp_dir
        )

        base_path = os.path.join(self.temp_dir, "TestAlgo", "TestModel")

        expected_paths = [
            "env.yaml",
            "README.md",
            "train/requirements.txt",
            "train/Dockerfile",
            "train/src/task.py",
            "infer/requirements.txt",
            "infer/Dockerfile",
            "infer/src/task.py",
            #"data/test_data/.gitkeep", FIX THIS
            "common/src/data/impl/local_training_util.py",
            "common/src/data/hedge_util.py",
            "common/src/util/config_extractor.py",
            "common/src/util/custom_encoder.py",
            "common/src/util/data_transformer.py",
            "common/src/util/exceptions.py",
            "common/src/util/infer_exception.py",
            "common/src/util/logger_util.py",
            "common/src/util/env_util.py",
            "common/src/util/logger_util.py",
            "common/src/ml/hedge_training.py",
            "common/src/ml/hedge_inference.py",
            "common/src/ml/hedge_status.py",
            "common/src/ml/hedge_api.py",
            "common/src/ml/watcher.py"
        ]

        for rel_path in expected_paths:
            full_path = os.path.join(base_path, rel_path)
            self.assertTrue(
                os.path.exists(full_path),
                f"Expected '{rel_path}' to exist but was not found at: {full_path}"
            )

        expected_dirs = [
            "train/src",
            "infer/src",
            "data/test_data",
            "common/src/util",
            "common/src/ml",
            "common/src/data"
            # ...
        ]
        for rel_dir in expected_dirs:
            full_dir = os.path.join(base_path, rel_dir)
            self.assertTrue(
                os.path.isdir(full_dir),
                f"Expected directory '{rel_dir}' to exist but was not found at: {full_dir}"
            )
        
        # This will test the execution of task.py script
        train_script = f"{base_path}/train/src/task.py"
        
        script_dir = os.path.dirname(os.path.dirname(base_path))
        
        # Set environment variable for the env.yaml file location
        env = os.environ.copy()
        
        # Run the script
        with subprocess.Popen([sys.executable, train_script], 
                                cwd=script_dir, 
                                stdout=subprocess.PIPE, 
                                stderr=subprocess.PIPE, 
                                text=True,
                                env=env) as proc:
            stdout, stderr = proc.communicate()
            
            self.assertEqual(proc.returncode, 0, 
                                f"Execution failed for {train_script}:\nSTDOUT: {stdout}\nSTDERR: {stderr}")



if __name__ == "__main__":
    unittest.main()
