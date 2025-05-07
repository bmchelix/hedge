
# Unit tests
Use the below commands to run python code related unit-tests along with coverage report

## 1. Build container 

`docker build  --label "Name=python-tests" -t python-tests:latest -f tests/Dockerfile .`

## 2. Run the container

`docker run --rm  -e LOCAL=True -v $(pwd)/tests/:/edge-iot/tests python-tests:latest`

## 3. Check the folder under /tests/coverage_report and browse for this file [class_index.html](coverage_report/class_index.html)

``/tests/coverage_report/class_index.html`


## Any new algorithm folders that gets added parallel to common/ folder, for e.g, simulation/ needs to be included in Dockerfile. 

`RUN mkdir -p /edge-iot/simulation/`

`COPY ${ROOT_FOLDER}/../simulation/ simulation/ `