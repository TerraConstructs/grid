#!/bin/bash

make db-reset
make build
make db-migrate
gridctl policy set --file examples/terraform/label-policy.json

# Create states with labels
gridctl state create prod-network --label env=prd --label region=us-east-1 --label project=core --label team=TIES
gridctl state create --force prod-cluster --label env=prd --label region=us-east-1 --label project=core --label team=TIES
gridctl state create --force prod-db --label env=prd --label region=us-east-1 --label project=core --label team=TIES
gridctl state create --force prod-app1 --label env=prd --label region=us-east-1 --label project=app1 --label team=FOO
gridctl state create --force prod-app2 --label env=prd --label region=us-east-1 --label project=app2 --label team=BAR
gridctl state create --force stage-app2 --label env=stg --label region=us-east-1 --label project=app2 --label team=BAR
gridctl state create --force stage-app1 --label env=stg --label region=us-east-1 --label project=app1 --label team=FOO
gridctl state create --force stage-db --label env=stg --label region=us-east-1 --label project=core --label team=TIES
gridctl state create --force stage-network --label env=stg --label region=us-east-1 --label project=core --label team=TIES
gridctl state create --force stage-cluster --label env=stg --label region=us-east-1 --label project=core --label team=TIES
# gridctl state list
gridctl dep add --from prod-network --to prod-cluster -o vpc_id
gridctl dep add --from prod-network --to prod-cluster -o subnet_ids
gridctl dep add --from stage-network --to stage-cluster -o subnet_ids
gridctl dep add --from stage-network --to stage-cluster -o vpc_id
gridctl dep add --from stage-network --to stage-db -o vpc_id
gridctl dep add --from prod-network --to prod-db -o vpc_id
# gridctl state list --label env=prd
gridctl dep add --from prod-cluster -o cluster_id --to prod-app1
gridctl dep add --from prod-cluster -o cluster_id --to prod-app2
gridctl dep add --from prod-db -o endpoint --to prod-app2
gridctl dep add --from prod-db -o endpoint --to prod-app1
gridctl dep add --from stage-cluster -o cluster_id --to stage-app1
gridctl dep add --from stage-cluster -o cluster_id --to stage-app2
gridctl dep add --from stage-db -o endpoint --to stage-app2
gridctl dep add --from stage-db -o endpoint --to stage-app1
