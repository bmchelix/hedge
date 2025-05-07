import os
import shutil

def copy_external_directory(external_folder, destination_folder):
    # Copy the entire directory
    try:
        shutil.copytree(external_folder, destination_folder, dirs_exist_ok=True)
        print(f"Successfully copied '{external_folder}' to '{destination_folder}'.")
    except Exception as e:
        print(f"Error while copying directory: {e}")
        raise Exception("Could not copy external directory to '{destination_folder}'")

def move_data_folder(project_dir):

    parent_dir = os.path.dirname(project_dir)

    if "{{cookiecutter.algo_type}}".lower() == "anomaly":
        external_folder = os.path.join(parent_dir, "byoa-pkg/hooks/data/anomaly")
    elif "{{cookiecutter.algo_type}}".lower() == "classification":
        external_folder = os.path.join(parent_dir, "byoa-pkg/hooks/data/classification")
    elif "{{cookiecutter.algo_type}}".lower() == "regression":
        external_folder = os.path.join(parent_dir, "byoa-pkg/hooks/data/regression")
    else:
        print("Invalid algorithm type '{cookiecutter.algo_type}'. Supported types are 'anomaly', 'classification', and'regression'.")
        return

    if not os.path.exists(external_folder):
        print(f"Source folder '{external_folder}' does not exist.")
        return

    destination_folder = os.path.join(project_dir, "{{cookiecutter.ml_model}}", "data/test_data")


    copy_external_directory(external_folder, destination_folder)

def move_common_folder(project_dir):
    common_libraries_folder = "{{cookiecutter.common_libraries_folder}}"
    if not os.path.exists(common_libraries_folder):
        print(f"Source folder '{common_libraries_folder}' does not exist.")
        return

    external_folder = "{{cookiecutter.common_libraries_folder}}"
    destination_folder = os.path.join(project_dir, "{{cookiecutter.ml_model}}", os.path.basename(external_folder))

    copy_external_directory(external_folder, destination_folder)

if __name__ == "__main__":
    project_dir = os.getcwd()

    move_common_folder(project_dir)
    print("Done with moving commons folder")
    move_data_folder(project_dir)
    print("Done with moving data zip file")
    


