# shinko-operator
Starts up the shinko app, as well as a shinko db running postgres.
Ready to work! Just run it on your cluster, where it is exposed on localhost through minikube tunnel.

## Description
Refer to https://github.com/Joeavaikath/Shinko for more information on what shinko is all about.

## Getting Started

- Install the CRD 
    ```
    make install
    ```
- Deploy the operator
    ```
    make deploy
    ```
- Run the CRD deployment
    ```
    cd example-crd-deploy
    ./deploy-crds.sh
    ```
    - Deploys the CRD for 
        1. shinko-app, 
        2. shinko-db and 
        3. runs a job to migrate the schema on the db to the latest image.

## How it works

- Installing the **CRD definition** allows your cluster to understand what it's definition is.
- Deploying the operator works in conjunction with this.
- When a **CRD resource** is deployed, operator sees it and its game time.
- Operator does what operator controllers are defined to do.
    - The reconciliation loops in the controllers run to ensure the status matches spec. 