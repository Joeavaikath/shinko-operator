#!/bin/sh

kubectl apply -f shinko-app.yaml
kubectl apply -f shinko-db-app.yaml
kubectl apply -f goose-migrations-job.yaml