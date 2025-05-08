class FeatureExtractor:
    timestamp = None
    algo_definition = None
    def __init__(self, data):
        if "mlModelConfig" in data and "mlDataSourceConfig" in data["mlModelConfig"]:
            self.data = data["mlModelConfig"]["mlDataSourceConfig"]
        else:
            self.data = data
        
        if "mlAlgoDefinition" in data:
            self.algo_definition = data["mlAlgoDefinition"]
        
        self.output_prediction_count = None
        self.input_context_count = None
        self.group_by_cols = False
        self.timestamp = None

        self.features_list = []
        self.input_features_list = []
        self.output_features_list = []
        self.categorical_input_list = []
        self.categorical_output_list = []
        self.numerical_input_list = []
        self.numerical_output_list = []
        self.extract_features()


    def get_data_object(self):
        return self.data

        
    def extract_features(self):
        # Extract the profile key
        profiles = self.data.get('featuresByProfile', {})
        
        features_dict = self.data.get("featureNameToColumnIndex", {})
        features_dict = {key: value for key, value in sorted(features_dict.items(), key=lambda item: item[1])}
        
        for profile_key, features in profiles.items():
            for key in features_dict.keys():
                # Find the feature in the features list that matches the current key
                feature = next((f for f in features if f"{profile_key}#{f['name']}" == key), None)
                
                if feature:
                    feature_name = f"{profile_key}#{feature['name']}"
                    self.features_list.append(feature_name)
                    
                    # Check if the feature is input or output
                    if feature.get('isInput'):
                        self.input_features_list.append(feature_name)
                        # Sort into categorical or numerical based on 'type'
                        if feature['type'] == "METRIC":
                            self.numerical_input_list.append(feature_name)
                        else:
                            self.categorical_input_list.append(feature_name)
                    
                    # Check if the feature is input or output
                    if feature.get('isOutput'):
                        self.output_features_list.append(feature_name)
                        # Sort into categorical or numerical based on 'type'
                        if feature['type'] == "METRIC":
                            self.numerical_output_list.append(feature_name)
                        else:
                            self.categorical_output_list.append(feature_name)
        
        # Extract timestamp if required
        if self.algo_definition and self.algo_definition.get("timeStampAttributeRequired", False):
            self.output_prediction_count = self.data.get("outputPredictionCount")
            self.input_context_count = self.data.get("inputContextCount")
            feature_name_to_column_index = self.data.get("featureNameToColumnIndex", {})            
            if feature_name_to_column_index:
                self.timestamp = list(feature_name_to_column_index.keys())[0]
                self.features_list.insert(0, self.timestamp)
                self.input_features_list.insert(0, self.timestamp)
                self.output_features_list.insert(0, self.timestamp)

        #Extract deviceID if required
        if self.algo_definition and self.algo_definition.get("groupByAttributesRequired", False):
            self.group_by_cols = self.data.get("groupOrJoinKeys", []) 
            for col in self.group_by_cols:
                self.features_list.insert(features_dict[col], col)  
                self.input_features_list.insert(features_dict[col], col)           
                self.output_features_list.insert(features_dict[col], col)                
            

    def get_features_list(self):
        return self.features_list

    def get_input_features_list(self):
        return self.input_features_list

    def get_output_features_list(self):
        return self.output_features_list

    def get_categorical_inputs_list(self):
        return self.categorical_input_list
    
    def get_categorical_outputs_list(self):
        return self.categorical_output_list

    def get_numerical_inputs_list(self):
        return self.numerical_input_list
    
    def get_numerical_outputs_list(self):
        return self.numerical_output_list
    
    def get_group_by_cols(self):
        return self.group_by_cols
