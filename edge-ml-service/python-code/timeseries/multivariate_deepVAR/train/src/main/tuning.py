from gluonts.mx.trainer import Trainer
from gluonts.dataset.split import split
from gluonts.mx.model.deepvar import DeepVAREstimator
from gluonts.evaluation import MultivariateEvaluator, backtest_metrics


class DeepVARTuningObjective:
    def __init__(self, dataset, output_prediction_count, freq, target_columns, metric_type="mean_wQuantileLoss"):
        self.dataset = dataset
        self.output_prediction_count = output_prediction_count
        self.freq = freq
        self.metric_type = metric_type
        self.target_columns = target_columns

        self.train, test_template = split(dataset, offset=-self.output_prediction_count)
        validation_instance = test_template.generate_instances(output_prediction_count=output_prediction_count)
        self.validation_dataset = validation_instance.dataset

    def get_params(self, trial) -> dict:
        return {
            "num_layers": trial.suggest_int("num_layers", 1, 5),
            "num_cells": trial.suggest_int("num_cells", 30, 100),
            "dropout_rate": trial.suggest_uniform("dropout_rate", 0.1, 0.5),
        }

    def __call__(self, trial):
        params = self.get_params(trial)
        estimator = DeepVAREstimator(
            num_layers=params["num_layers"],
            num_cells=params["num_cells"],
            dropout_rate=params["dropout_rate"],
            target_dim=len(self.target_columns),
            output_prediction_count=self.output_prediction_count,
            freq=self.freq,
            trainer=Trainer(epochs=10, learning_rate=1e-3, hybridize=True)
        )

        predictor = estimator.train(self.train, cache_data=True)

        quantiles = [0.6, 0.7]  # Adjusted quantiles for confidence intervals
        agg_metrics, _ = backtest_metrics(
            test_dataset=self.validation_dataset,
            predictor=predictor,
            evaluator=MultivariateEvaluator(quantiles=quantiles)
        )
        return agg_metrics[self.metric_type]
