package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type SqoushStackProps struct {
	awscdk.StackProps
}

func NewSqoushStack(scope constructs.Construct, id string, props *SqoushStackProps) awscdk.Stack {
	var stackProps awscdk.StackProps
	if props != nil {
		stackProps = props.StackProps
	}

	stack := awscdk.NewStack(scope, &id, &stackProps)

	ddbTable := awsdynamodb.NewTable(stack, jsii.String("SqoushTable"), &awsdynamodb.TableProps{
		TableName:   jsii.String("sqoush-app"),
		PartitionKey: &awsdynamodb.Attribute{Name: jsii.String("PK"), Type: awsdynamodb.AttributeType_STRING},
		SortKey:      &awsdynamodb.Attribute{Name: jsii.String("SK"), Type: awsdynamodb.AttributeType_STRING},
		BillingMode: awsdynamodb.BillingMode_PAY_PER_REQUEST,
		RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
	})

	ddbTable.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("GSI1"),
		PartitionKey: &awsdynamodb.Attribute{Name: jsii.String("GSI1PK"), Type: awsdynamodb.AttributeType_STRING},
		SortKey:      &awsdynamodb.Attribute{Name: jsii.String("GSI1SK"), Type: awsdynamodb.AttributeType_STRING},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	lambdaFn := awslambda.NewFunction(stack, jsii.String("SqoushApi"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_GO_1_X(),
		Handler: jsii.String("main"),
		Code:    awslambda.Code_FromAsset(jsii.String("../"), nil),
		Environment: &map[string]*string{
			"APP":              jsii.String("prod"),
			"DYNAMODB_TABLE":   ddbTable.TableName(),
			"RECAPTCHA_SITE_KEY": jsii.String(os.Getenv("RECAPTCHA_SITE_KEY")),
			"RECAPTCHA_SECRET_KEY": jsii.String(os.Getenv("RECAPTCHA_SECRET_KEY")),
		},
	})

	ddbTable.GrantReadWriteData(lambdaFn)

	awsapigateway.NewLambdaRestApi(stack, jsii.String("SqoushApiGateway"), &awsapigateway.LambdaRestApiProps{
		Handler: lambdaFn,
	})

	awscdk.NewCfnOutput(stack, jsii.String("TableName"), &awscdk.CfnOutputProps{Value: ddbTable.TableName()})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)
	NewSqoushStack(app, "SqoushStack", &SqoushStackProps{})
	app.Synth(nil)
}
