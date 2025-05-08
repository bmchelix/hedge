# Cookiecutter Template for ML Projects and also onboarding BYOA

This repository provides a customizable project structure for machine learning (ML) workflows using [Cookiecutter](https://cookiecutter.readthedocs.io/). The template is designed to streamline the creation of ML model training and inference pipelines, with Docker integration and reusable task classes.

## Why Use Cookiecutter?

1. **Consistency**: Ensures a uniform project structure for ML workflows.
2. **Reusability**: Easily create new projects with pre-defined templates.
3. **Customization**: Generate files and folders dynamically based on user inputs.
4. **Simplicity**: Quickly scaffold projects without repetitive manual setup.

## Prerequisites

- Python 3.6 or higher
- [Cookiecutter](https://cookiecutter.readthedocs.io/en/stable/installation.html) installed using pip install

## Installation

Install Cookiecutter using pip:

```bash
pip install cookiecutter==2.6.0
```


## Usage

1. Download the artifact byoa-pkg.zip, unzip it and reference it directly.

   ```bash
   cd <PARENT_FOLDER_OF_byoa-pkg>
   
   cookiecutter -f byoa-pkg
   ```
   - ***Use -f to override the existing folders***

2. Provide inputs when prompted, such as:
   - `ml_algo`: Name of the main project folder (e.g., `MyAlgorithm`).
   - `ml_model`: Name of the model folder (e.g., `MyModel`).
   - `base_image`: Docker base image (e.g., `python:3.9-slim`).
   - `common_package`: Common package name (e.g., `common`).
   - `training_class_name`: Class name for the training task (e.g., `TrainTask`).
   - `inference_class_name`: Class name for the inference task (e.g., `InferTask`).
   - Select `algo_type`: Choose Algorithm Type (e.g., `1. Anomaly`, `2.Classification`, `3.Regression`).

3. Navigate to the generated project directory and start working on your ML tasks.

### Example

**Input:**
```
ml_algo (ALGORITHM NAME: [regression]): regression

ml_model (MODEL NAME: [linear]): linear

base_image (BASE IMAGE: [hedge-ml-python-base:internal]): 
hedge-ml-python-base:internal

common_libraries_folder (FULL PATH TO COMMON LIBRARY: [../common]): /Users/git/edge-iot/edge-ml-service/python-code/common

training_class_name (TRAINING CLASS NAME: [RegressionTrainingTask]): RegressionTrainingTask

infer_class_name (INFERENCE CLASS NAME: [RegressionInferenceTask]): RegressionInferenceTask

Select algo_type 1 - Anomaly
    2 - Classification
    3 - Regression
    Choose from [1/2/3] (1): 3
```

**Generated Structure:**

```
{{cookiecutter.ml_algo}}
├── {{cookiecutter.ml_model}}
|   ├── common
|   │   ├──..... common libraries
|   ├── data
|   │   ├──test_data
|   │       └── ..... placeholder for training zip file for local testing
|   ├── train
|   │   ├── requirements.txt
|   │   ├── Dockerfile
|   │   └── src
|   │       └── task.py
|   ├── infer
|   │   ├── requirements.txt
|   │   ├── Dockerfile
|   │   └── src
|   │       └── task.py
│   └── env.yaml
└── README.md

```


## Docker Integration

Each module (training and inference) includes:

- **`Dockerfile`**: Defines the Docker container configuration.
- **`requirements.txt`**: Lists dependencies to install in the container.

## Customization

Modify the `cookiecutter.json` file to add more variables or adjust the template structure as needed. The placeholders `{{cookiecutter.<variable_name>}}` in the template files will be replaced with user-provided values.
