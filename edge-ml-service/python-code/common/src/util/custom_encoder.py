import numpy as np
import pandas as pd
from sklearn.preprocessing import OneHotEncoder

class CustomOneHotEncoder(OneHotEncoder):
    def __init__(self, **kwargs):
        super().__init__(sparse_output=False, handle_unknown='ignore', **kwargs)
        self.categorical_columns = None
        self.numeric_columns = None
        self.all_columns = None
        self.updated_feature_names = []

    def fit(self, X, y=None):
        # Identify categorical and numeric columns
        self.categorical_columns = X.select_dtypes(include=['object', 'category']).columns
        self.numeric_columns = X.select_dtypes(include=[np.number]).columns
        self.all_columns = X.columns

        # Fit the OneHotEncoder on the categorical columns
        super().fit(X[self.categorical_columns], y)
        return self

    def transform(self, X):
        # Transform the categorical columns using OneHotEncoder
        encoded_data = super().transform(X[self.categorical_columns])

        # Create a DataFrame for the encoded data with appropriate column names
        encoded_df = pd.DataFrame(encoded_data, columns=self.get_feature_names_out(self.categorical_columns))

        # Add columns to handle unknown categories
        for column in self.categorical_columns:
            unknown_col_name = f'{column}_unknown'
            encoded_df[unknown_col_name] = np.zeros(len(X))  # Add the unknown column

            known_categories = set(self.categories_[self.categorical_columns.get_loc(column)])
            for idx, category in enumerate(X[column]):
                if category not in known_categories:
                    encoded_df.loc[idx, unknown_col_name] = 1  # Mark unknown category

        # Drop the original categorical columns and combine with the encoded data
        non_categorical_data = X.drop(columns=self.categorical_columns)

        # Concatenate the encoded categorical columns and non-categorical columns
        transformed_data = pd.concat([encoded_df, non_categorical_data.reset_index(drop=True)], axis=1)

        # Update the feature names to include the new unknown columns
        self.updated_feature_names = list(encoded_df.columns) + list(non_categorical_data.columns)

        return transformed_data

    def fit_transform(self, X, y=None):
        # Fit and transform the data, also updates the feature names
        return self.fit(X, y).transform(X)
