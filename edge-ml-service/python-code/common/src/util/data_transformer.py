import pandas as pd
from sklearn.preprocessing import StandardScaler
from sklearn.compose import ColumnTransformer

from common.src.util.custom_encoder import CustomOneHotEncoder


class DataFramePreprocessor:
    def __init__(self):
        self.preprocessor = None
        self.numeric_features = None
        self.categorical_features = None

    def fit(self, df: pd.DataFrame):
        """
        Fit the preprocessors on the training data.

        Args:
            df (pd.DataFrame): The input training dataframe.
        """
        try:
            # Identify numeric and categorical columns
            self.numeric_features = df.select_dtypes(include=['int64', 'float64']).columns.tolist()
            self.categorical_features = df.select_dtypes(include=['object', 'category']).columns.tolist()

            # Define transformers for numeric and categorical features
            transformers = []
            if self.numeric_features:
                numeric_transformer = StandardScaler()
                transformers.append(('num', numeric_transformer, self.numeric_features))
            if self.categorical_features:
                categorical_transformer = CustomOneHotEncoder()
                transformers.append(('cat', categorical_transformer, self.categorical_features))

            # Create a preprocessor using ColumnTransformer
            self.preprocessor = ColumnTransformer(transformers=transformers)

            # Fit the preprocessor
            self.preprocessor.fit(df)
        except Exception as e:
            raise ValueError(f"Failed to fit the preprocessor: {e}")

    def transform(self, df: pd.DataFrame) -> pd.DataFrame:
        """
        Transform the data using the fitted preprocessors.

        Args:
            df (pd.DataFrame): The input dataframe.

        Returns:
            pd.DataFrame: The transformed dataframe.
        """
        if self.preprocessor is None:
            raise ValueError("The preprocessor has not been fitted. Call fit first.")

        # Transform the data
        df_transformed = self.preprocessor.transform(df)

        # Create a list to store feature names
        feature_names = []

        # Append numeric features
        if self.numeric_features:
            feature_names += self.numeric_features

        # Append categorical features (after One-Hot Encoding), including the unknown columns
        if self.categorical_features:
            categorical_feature_names = self.preprocessor.transformers_[1][1].updated_feature_names
            feature_names += categorical_feature_names

        # Ensure that the number of feature names matches the transformed data shape
        if len(feature_names) != df_transformed.shape[1]:
            raise ValueError(
                f"Mismatch between transformed data columns ({df_transformed.shape[1]}) and feature names ({len(feature_names)}).")

        # Create a DataFrame with the transformed data and the correct feature names
        df_transformed = pd.DataFrame(df_transformed, columns=feature_names)

        return df_transformed

    def fit_transform(self, df: pd.DataFrame) -> pd.DataFrame:
        """
        Fit the preprocessors on the training data and transform it.

        Args:
            df (pd.DataFrame): The input training dataframe.

        Returns:
            pd.DataFrame: The transformed training dataframe.
        """
        self.fit(df)
        return self.transform(df)

    def get_preprocessor(self) -> ColumnTransformer:
        """
        Get the fitted ColumnTransformer object.

        Returns:
            ColumnTransformer: The fitted ColumnTransformer object.
        """
        if self.preprocessor is None:
            raise ValueError("The preprocessor has not been fitted. Call fit first.")
        return self.preprocessor
    
    def get_standard_scaler(self) -> StandardScaler:
        """
        Extract the StandardScaler from the fitted ColumnTransformer.

        Returns:
            StandardScaler: The StandardScaler used in the preprocessing.
        """
        if self.preprocessor is None:
            raise ValueError("The preprocessor has not been fitted. Call fit first.")
        return self.preprocessor.named_transformers_['num']