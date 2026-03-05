#!/bin/bash

echo "Creating DynamoDB table locally..."

aws dynamodb create-table \
    --table-name CyclingTrainerData \
    --attribute-definitions \
        AttributeName=PK,AttributeType=S \
        AttributeName=SK,AttributeType=S \
    --key-schema \
        AttributeName=PK,KeyType=HASH \
        AttributeName=SK,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST \
    --endpoint-url http://localhost:8001 \
    --region us-east-1

echo "Table created successfully."
