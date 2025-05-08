# {{cookiecutter.ml_algo}} - {{cookiecutter.ml_model}} {style=text-transform:uppercase}

This repository contains the base structure for creating machine learning model training and inference pipelines.

## Structure

- **{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/train**: Contains files for training the model.
- **{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/infer**: Contains files for inference end-points.
- **{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/common**: Contains files for common libraries.

## Usage Instructions:

### Code Updates 
- **{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/train/src/task.py**: Update the training related code where ever TODO section is updated
- **{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/infer/src/task.py**: Update the inference related code where ever TODO section is updated

### Local configuration
* Update `ENV LOCAL=False` to `ENV LOCAL=True`
* Update requirements.txt file to contain necessary requirements
* Modify the training zip file under `{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/data/test_data/{algorithm-type}.zip` 
  * In this zip you will find the file structure, sample data, and configuration file needed for training and inference. 
  * Modify as needed but you can test with our zip


### Training
```bash
docker build --no-cache -t {{cookiecutter.ml_algo}}_train -f {{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/train/Dockerfile .
docker run -e MODELDIR=/tmp/res/edge/models -e TRAINING_FILE_ID=/tmp/res/edge/test_data/training.zip -v ${PWD}/{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/data:/tmp/res/edge {{cookiecutter.ml_algo}}_train

```

### Inference 
```bash
docker build --no-cache -t {{cookiecutter.ml_algo}}_infer -f {{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/infer/Dockerfile .
docker run -p 55000:55000 -e MODELDIR=/tmp/res/edge/models -v ${PWD}/{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/data:/tmp/res/edge {{cookiecutter.ml_algo}}_infer
```
