package auth

import (
	"context"
	"fmt"
	"time"

	"cycling-trainer-agent-ai/backend/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func UpdateProfile(ctx context.Context, db *dynamodb.Client, tableName string, email string, ftp float64, weight float64) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	timestamp := time.Now().Format(time.RFC3339)

	// Update Profile
	_, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + email},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
		UpdateExpression: aws.String("SET ftp = :f, weight = :w"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":f": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", ftp)},
			":w": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", weight)},
		},
	})
	if err != nil {
		return err
	}

	// Record History
	history := models.FTPRecord{
		PK:        "USER#" + email,
		SK:        "FTP#" + timestamp,
		FTP:       ftp,
		Weight:    weight,
		Timestamp: timestamp,
	}
	item, _ := attributevalue.MarshalMap(history)
	_, err = db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	return err
}

func GetProfile(ctx context.Context, db *dynamodb.Client, tableName string, email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + email},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		return nil, err
	}
	if result.Item == nil {
		return nil, fmt.Errorf("perfil no encontrado")
	}

	var user models.User
	err = attributevalue.UnmarshalMap(result.Item, &user)
	return &user, err
}
