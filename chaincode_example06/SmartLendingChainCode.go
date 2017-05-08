package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

var logger = shim.NewLogger("CLDChaincode") // TODO : Find out what this is

//==============================================================================================================================
//	 Participant types
//==============================================================================================================================
//CURRENT WORKAROUND USES ROLES CHANGE WHEN OWN USERS CAN BE CREATED SO THAT IT READ 1, 2, 3, 4, 5
const BORROWER = "borrower"
const DEALER = "dealer"
const LENDER = "lender"

//==============================================================================================================================
//	 Status types - Loan Application
//==============================================================================================================================
const STATE_APPLIED = 0
const STATE_QUOTATIONS_RECEIVED = 1
const STATE_BID_ACCEPTED = 2
const STATE_BID_REJECTED = 3
const STATE_PERFORMING = 4
const STATE_NON_PERFORMING = 5

//==============================================================================================================================
//	Status types - Lender accept status of an application
//==============================================================================================================================
const LENDER_ACCEPT_APPLICATION = 1
const LENDER_REJECT_APPLICATION = 0

//==============================================================================================================================
//	Status types - Payment status
//==============================================================================================================================
const STATE_NOT_DEMANDED = 0
const STATE_DEMANDED = 1
const STATE_RECOVERED = 2
const STATE_MISSED = 3

//==============================================================================================================================
//	 Structure Definitions
//==============================================================================================================================
//	Chaincode - A blank struct for use with Shim (A HyperLedger included go file used for get/put state
//				and other HyperLedger functions)
//==============================================================================================================================
type SmartLendingChaincode struct {
}

//==============================================================================================================================
//	Models
//==============================================================================================================================

type LoanApplication struct {
	ApplicationNumber string
	AccountNumber     int
	Make              string
	Model             string
	LoanAmount        float64
	SSN               string
	Age               int
	MonthlyIncome     float64
	CreditScore       int
	Status            int
	Tenure            int
	Transactions      []TransactionMetadata
	Quotations        []BiddingDetails
	RepaymentSchedule []PaymentDetail
}

type EvaluationParams struct {
	ApplicationNumber string
	LoanAmount        float64
	SSN               string
	Age               int
	MonthlyIncome     float64
	CreditScore       int
	Tenure            int
}

type BiddingDetails struct {
	ApplicationNumber       string
	BiddingNumber           int
	BiddingDate             time.Time
	LenderId                int
	SanctionedAmount        float64
	InterestType            string
	InterestRate            float64
	Tenure                  int
	ApplicationAcceptStatus int
	RejectionReason         string
	IsWinningBid            bool
}

type TransactionMetadata struct {
	ApplicationState     int
	TransactionId        string
	TransactionTimestamp string
	TransactionDate      time.Time
	CallerMetadata       []byte
}

type PaymentDetail struct {
	InstallmentNumber int
	PrincipalAmount   float64
	InterestAmount    float64
	TotalEMI          float64
	RepaymentStatus   int
	RepaymentDate     string
	Metadata          TransactionMetadata
}

//==============================================================================================================================
//	Init Function - Called when the user deploys the chaincode
//==============================================================================================================================
func (t *SmartLendingChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	fmt.Println("Smart lending chaincode initiated")

	return nil, nil
}

//==============================================================================================================================
//	Query Function - Called when the user queries the chaincode
//==============================================================================================================================

func (t *SmartLendingChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function == "GetApplicationDetails" {
		fmt.Println("Calling GetLoanApplicationDetails")
		return t.GetLoanApplicationDetails(stub, args[0])
	}
	fmt.Println("Function not found")
	return nil, errors.New("No query functions")
}

//==============================================================================================================================
//	Invoke Function - Called when the user invokes the chaincode
//==============================================================================================================================

func (t *SmartLendingChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function == "CreateLoanApplication" {
		return t.CreateLoanApplication(stub, args)
	} else if function == "ConfirmBid" {
		return t.ConfirmBid(stub, args)
	} else if function == "ChangePaymentStatus" {
		return t.ChangePaymentStatus(stub, args)
	}
	fmt.Println("Function not found")
	return nil, errors.New("Invalid invoke function name")
}

//==============================================================================================================================
//	 POC related invoke functions for Sprint 1 and 2
//==============================================================================================================================
func (t *SmartLendingChaincode) CreateLoanApplication(stub shim.ChaincodeStubInterface, applicationArgs []string) ([]byte, error) {

	// Validate the application details
	if applicationArgs[0] == "" {
		fmt.Printf("Invalid application")
		return nil, errors.New("Invalid application")
	}

	// Check if the application already exist
	bytes, err := stub.GetState(applicationArgs[0])
	if bytes != nil {
		return nil, errors.New("Application already exist")
	}

	// Construct the application details
	var applicationNumber string = applicationArgs[0]
	var make string = applicationArgs[1]
	var model string = applicationArgs[2]
	loanAmount, err := strconv.ParseFloat(applicationArgs[3], 64)
	var ssn string = applicationArgs[4]
	age, err := strconv.Atoi(applicationArgs[5])
	monthlyIncome, err := strconv.ParseFloat(applicationArgs[6], 64)
	creditScore, err := strconv.Atoi(applicationArgs[7])
	loanTenure, err := strconv.Atoi(applicationArgs[8])

	applicationDetails := LoanApplication{ApplicationNumber: applicationNumber, Make: make, Model: model, LoanAmount: loanAmount, SSN: ssn, Age: age, MonthlyIncome: monthlyIncome, CreditScore: creditScore, Status: STATE_APPLIED}

	// Save the loan application
	applicationDetails = t.SaveApplicationDetails(stub, applicationDetails)

	// Prepare the evaluation parameters
	evaluationParams := EvaluationParams{ApplicationNumber: applicationNumber, LoanAmount: loanAmount, SSN: ssn, Age: age, MonthlyIncome: monthlyIncome, CreditScore: creditScore, Tenure: loanTenure}

	// Get quotes from lenders
	quoteFromLender1 := t.GetQuoteFromLender1(evaluationParams)
	quoteFromLender2 := t.GetQuoteFromLender2(evaluationParams)
	quoteFromLender3 := t.GetQuoteFromLender3(evaluationParams)
	quoteFromLender4 := t.GetQuoteFromLender4(evaluationParams)

	// Add the quotations to the loan application
	var quotes []BiddingDetails
	quotes = append(quotes, quoteFromLender1)
	quotes = append(quotes, quoteFromLender2)
	quotes = append(quotes, quoteFromLender3)
	quotes = append(quotes, quoteFromLender4)
	applicationDetails.Quotations = quotes
	applicationDetails.Status = STATE_QUOTATIONS_RECEIVED
	applicationDetails = t.SaveApplicationDetails(stub, applicationDetails)

	bytes, err = json.Marshal(applicationDetails)

	return bytes, err
}

func (t *SmartLendingChaincode) ConfirmBid(stub shim.ChaincodeStubInterface, applicationArgs []string) ([]byte, error) {

	bytes, err := stub.GetState(applicationArgs[0])
	biddingNumber, err := strconv.Atoi(applicationArgs[1])
	bidStatus, err := strconv.Atoi(applicationArgs[2])
	var applicationDetails LoanApplication

	err = json.Unmarshal(bytes, &applicationDetails)

	if err != nil {
		fmt.Println("Error while coverting JSON: " + err.Error())
	}

	applicationDetails.Status = bidStatus

	for i := 0; i < len(applicationDetails.Quotations); i++ {
		if applicationDetails.Quotations[i].BiddingNumber == biddingNumber && bidStatus == STATE_BID_ACCEPTED {
			applicationDetails.Quotations[i].IsWinningBid = true
			applicationDetails.AccountNumber = t.GenerateAccountNumber()
			applicationDetails.RepaymentSchedule = t.GenerateRepaymentSchedule(applicationDetails.Quotations[i])
		}
	}

	fmt.Println("after setting bid")

	applicationDetails = t.SaveApplicationDetails(stub, applicationDetails)

	bytes, err = json.Marshal(applicationDetails)

	return bytes, err
}

func (t *SmartLendingChaincode) ChangePaymentStatus(stub shim.ChaincodeStubInterface, applicationArgs []string) ([]byte, error) {

	// Gather the inputs
	applicationNumber := applicationArgs[0]
	installmentNumber, err := strconv.Atoi(applicationArgs[2])
	repaymentStatus, err := strconv.Atoi(applicationArgs[3])

	// Get the application details
	var applicationDetails LoanApplication
	bytes, err := stub.GetState(applicationNumber)
	err = json.Unmarshal(bytes, &applicationDetails)

	// Loop through the repayment schedule and change the payment status
	for i := 0; i < len(applicationDetails.RepaymentSchedule); i++ {
		if applicationDetails.RepaymentSchedule[i].InstallmentNumber == installmentNumber {
			applicationDetails.RepaymentSchedule[i].RepaymentStatus = repaymentStatus
			var metadata TransactionMetadata
			metadata.TransactionId = stub.GetTxID()
			txnTimeStamp, err := stub.GetTxTimestamp()
			if err == nil {
				metadata.TransactionTimestamp = txnTimeStamp.String()
			}
			metadata.CallerMetadata, err = stub.GetCallerMetadata()
			metadata.TransactionDate = time.Now()
			applicationDetails.RepaymentSchedule[i].Metadata = metadata
			break
		}
	}

	// Get the revised loan application status
	applicationDetails = t.CheckLoanDefaultStatus(applicationDetails)
	applicationDetails = t.SaveApplicationDetails(stub, applicationDetails)

	bytes, err = json.Marshal(applicationDetails)

	return bytes, err
}

func (t *SmartLendingChaincode) GetLoanApplicationDetails(stub shim.ChaincodeStubInterface, applicationNumber string) ([]byte, error) {

	bytes, err := stub.GetState(applicationNumber)
	if err != nil {
		return nil, err
	}
	if bytes == nil {
		return nil, errors.New("Could not find application")
	}
	return bytes, err
}

//==============================================================================================================================
//	 Private functions
//==============================================================================================================================

func (t *SmartLendingChaincode) SaveApplicationDetails(stub shim.ChaincodeStubInterface, applicationDetails LoanApplication) LoanApplication {

	bytes, err := json.Marshal(applicationDetails)
	err = stub.PutState(applicationDetails.ApplicationNumber, bytes)

	if err == nil {

		// Get all the previous transactions made
		var transactions []TransactionMetadata
		for i := 0; i < len(applicationDetails.Transactions); i++ {
			transactions = append(transactions, applicationDetails.Transactions[i])
		}

		// Get the transaction metedata of the current transaction
		metadata := t.GetTransactionMetadata(stub, applicationDetails)
		transactions = append(transactions, metadata)
		applicationDetails.Transactions = transactions
	}

	bytes, err = json.Marshal(applicationDetails)
	err = stub.PutState(applicationDetails.ApplicationNumber, bytes)

	return applicationDetails
}

func (t *SmartLendingChaincode) GetTransactionMetadata(stub shim.ChaincodeStubInterface, applicationDetails LoanApplication) TransactionMetadata {
	var metadata TransactionMetadata
	metadata.ApplicationState = applicationDetails.Status
	metadata.TransactionId = stub.GetTxID()
	metadata.TransactionDate = time.Now()
	txnTimeStamp, err := stub.GetTxTimestamp()
	if err == nil {
		metadata.TransactionTimestamp = txnTimeStamp.String()
	}
	callerMetadata, err := stub.GetCallerMetadata()

	metadata.CallerMetadata = callerMetadata
	return metadata
}

func (t *SmartLendingChaincode) GetQuoteFromLender1(evaluationParams EvaluationParams) BiddingDetails {

	var bidDetails BiddingDetails
	bidDetails.ApplicationNumber = evaluationParams.ApplicationNumber

	// ==================================================================
	// Logic to determine whether to accept the application or reject it
	// ==================================================================
	if evaluationParams.CreditScore < 300 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting credit score requirements"
	} else if evaluationParams.Age < 18 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting age requirements"
	} else if utf8.RuneCountInString(evaluationParams.SSN) != 7 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Invalid SSN"
	} else if evaluationParams.MonthlyIncome < 1000.00 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting monthly income requirements"
	} else {
		// ==================================================================
		// Logic to construct the bid if the lender accepts the application
		// ==================================================================
		bidDetails.ApplicationAcceptStatus = LENDER_ACCEPT_APPLICATION
		bidDetails.BiddingNumber = t.GenerateBiddingNumber()
		bidDetails.BiddingDate = time.Now()
		bidDetails.LenderId = 1
		bidDetails.SanctionedAmount = evaluationParams.LoanAmount
		bidDetails.Tenure = evaluationParams.Tenure
		bidDetails.InterestType = "simple"

		// Calculate interest rate
		var baseRate float32 = 5.0
		var delta float32 = 0.0
		if evaluationParams.CreditScore < 700 && evaluationParams.CreditScore > 500 {
			delta = delta + 0.25
		} else if evaluationParams.CreditScore < 500 && evaluationParams.CreditScore > 300 {
			delta = delta + 0.50
		}

		if evaluationParams.Age > 30 && evaluationParams.Age < 50 {
			delta = delta + 0.25
		} else if evaluationParams.Age > 50 {
			delta = delta + 0.50
		}

		if evaluationParams.MonthlyIncome > 1000 && evaluationParams.MonthlyIncome < 3000 {
			delta = delta + 0.50
		} else if evaluationParams.MonthlyIncome > 3000 {
			delta = delta + 0.25
		}

		finalRate := baseRate + delta
		bidDetails.InterestRate = float64(finalRate)
		bidDetails.IsWinningBid = false
	}

	return bidDetails
}

func (t *SmartLendingChaincode) GetQuoteFromLender2(evaluationParams EvaluationParams) BiddingDetails {

	var bidDetails BiddingDetails
	bidDetails.ApplicationNumber = evaluationParams.ApplicationNumber

	// ==================================================================
	// Logic to determine whether to accept the application or reject it
	// ==================================================================
	if evaluationParams.CreditScore < 300 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting credit score requirements"
	} else if evaluationParams.Age < 18 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting age requirements"
	} else if utf8.RuneCountInString(evaluationParams.SSN) != 7 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Invalid SSN"
	} else if evaluationParams.MonthlyIncome < 1000.00 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting monthly income requirements"
	} else {
		// ==================================================================
		// Logic to construct the bid if the lender accepts the application
		// ==================================================================
		bidDetails.ApplicationAcceptStatus = LENDER_ACCEPT_APPLICATION
		bidDetails.BiddingNumber = t.GenerateBiddingNumber()
		bidDetails.BiddingDate = time.Now()
		bidDetails.LenderId = 2
		bidDetails.SanctionedAmount = evaluationParams.LoanAmount
		bidDetails.Tenure = evaluationParams.Tenure
		bidDetails.InterestType = "floating"

		// Calculate interest rate
		var baseRate float32 = 5.0
		var delta float32 = 0.0
		if evaluationParams.CreditScore < 700 && evaluationParams.CreditScore > 500 {
			delta = delta + 0.25
		} else if evaluationParams.CreditScore < 500 && evaluationParams.CreditScore > 300 {
			delta = delta + 0.50
		}

		if evaluationParams.Age > 30 && evaluationParams.Age < 50 {
			delta = delta + 0.25
		} else if evaluationParams.Age > 50 {
			delta = delta + 0.50
		}

		if evaluationParams.MonthlyIncome > 1000 && evaluationParams.MonthlyIncome < 3000 {
			delta = delta + 0.50
		} else if evaluationParams.MonthlyIncome > 3000 {
			delta = delta + 0.25
		}

		finalRate := baseRate + delta
		bidDetails.InterestRate = float64(finalRate)
		bidDetails.IsWinningBid = false
	}

	return bidDetails
}

func (t *SmartLendingChaincode) GetQuoteFromLender3(evaluationParams EvaluationParams) BiddingDetails {

	var bidDetails BiddingDetails
	bidDetails.ApplicationNumber = evaluationParams.ApplicationNumber

	// ==================================================================
	// Logic to determine whether to accept the application or reject it
	// ==================================================================
	if evaluationParams.CreditScore < 300 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting credit score requirements"
	} else if evaluationParams.Age < 18 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting age requirements"
	} else if utf8.RuneCountInString(evaluationParams.SSN) != 7 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Invalid SSN"
	} else if evaluationParams.MonthlyIncome < 1000.00 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting monthly income requirements"
	} else {
		// ==================================================================
		// Logic to construct the bid if the lender accepts the application
		// ==================================================================
		bidDetails.ApplicationAcceptStatus = LENDER_ACCEPT_APPLICATION
		bidDetails.BiddingNumber = t.GenerateBiddingNumber()
		bidDetails.BiddingDate = time.Now()
		bidDetails.LenderId = 3
		bidDetails.SanctionedAmount = evaluationParams.LoanAmount
		bidDetails.Tenure = evaluationParams.Tenure
		bidDetails.InterestType = "simple"

		// Calculate interest rate
		var baseRate float32 = 5.0
		var delta float32 = 0.0
		if evaluationParams.CreditScore < 700 && evaluationParams.CreditScore > 500 {
			delta = delta + 0.25
		} else if evaluationParams.CreditScore < 500 && evaluationParams.CreditScore > 300 {
			delta = delta + 0.50
		}

		if evaluationParams.Age > 30 && evaluationParams.Age < 50 {
			delta = delta + 0.25
		} else if evaluationParams.Age > 50 {
			delta = delta + 0.50
		}

		if evaluationParams.MonthlyIncome > 1000 && evaluationParams.MonthlyIncome < 3000 {
			delta = delta + 0.50
		} else if evaluationParams.MonthlyIncome > 3000 {
			delta = delta + 0.25
		}

		finalRate := baseRate + delta
		bidDetails.InterestRate = float64(finalRate)
		bidDetails.IsWinningBid = false
	}

	return bidDetails
}

func (t *SmartLendingChaincode) GetQuoteFromLender4(evaluationParams EvaluationParams) BiddingDetails {

	var bidDetails BiddingDetails
	bidDetails.ApplicationNumber = evaluationParams.ApplicationNumber

	// ==================================================================
	// Logic to determine whether to accept the application or reject it
	// ==================================================================
	if evaluationParams.CreditScore < 300 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting credit score requirements"
	} else if evaluationParams.Age < 18 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting age requirements"
	} else if utf8.RuneCountInString(evaluationParams.SSN) != 7 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Invalid SSN"
	} else if evaluationParams.MonthlyIncome < 1000.00 {
		bidDetails.ApplicationAcceptStatus = LENDER_REJECT_APPLICATION
		bidDetails.RejectionReason = "Not meeting monthly income requirements"
	} else {
		// ==================================================================
		// Logic to construct the bid if the lender accepts the application
		// ==================================================================
		bidDetails.ApplicationAcceptStatus = LENDER_ACCEPT_APPLICATION
		bidDetails.BiddingNumber = t.GenerateBiddingNumber()
		bidDetails.BiddingDate = time.Now()
		bidDetails.LenderId = 4
		bidDetails.SanctionedAmount = evaluationParams.LoanAmount
		bidDetails.Tenure = evaluationParams.Tenure
		bidDetails.InterestType = "floating"

		// Calculate interest rate
		var baseRate float32 = 5.0
		var delta float32 = 0.0
		if evaluationParams.CreditScore < 700 && evaluationParams.CreditScore > 500 {
			delta = delta + 0.25
		} else if evaluationParams.CreditScore < 500 && evaluationParams.CreditScore > 300 {
			delta = delta + 0.50
		}

		if evaluationParams.Age > 30 && evaluationParams.Age < 50 {
			delta = delta + 0.25
		} else if evaluationParams.Age > 50 {
			delta = delta + 0.50
		}

		if evaluationParams.MonthlyIncome > 1000 && evaluationParams.MonthlyIncome < 3000 {
			delta = delta + 0.50
		} else if evaluationParams.MonthlyIncome > 3000 {
			delta = delta + 0.25
		}

		finalRate := baseRate + delta
		bidDetails.InterestRate = float64(finalRate)
		bidDetails.IsWinningBid = false
	}

	return bidDetails
}

func (t *SmartLendingChaincode) GenerateBiddingNumber() int {
	var biddingNumber int = 0

	// TODO : Store max bid number used in ledger and return the next number and remove random generation
	biddingNumber = rand.Intn(100000)

	return biddingNumber
}

func (t *SmartLendingChaincode) GenerateAccountNumber() int {
	var accountNumber int = 0

	// TODO : Store max bid number used in ledger and return the next number and remove random generation
	accountNumber = rand.Intn(100000)

	return accountNumber
}

func (t *SmartLendingChaincode) GenerateRepaymentSchedule(winningQuotation BiddingDetails) []PaymentDetail {
	var repaymentSchedule []PaymentDetail
	var noOfInstallments int

	// Construct the repayment RepaymentSchedule
	noOfInstallments = winningQuotation.Tenure * 12

	for i := 0; i < noOfInstallments; i++ {
		// Construct the installment details
		var installmentDetail PaymentDetail

		installmentDetail.InstallmentNumber = i + 1
		installmentDetail.PrincipalAmount = winningQuotation.SanctionedAmount / float64(noOfInstallments)
		installmentDetail.InterestAmount = (installmentDetail.PrincipalAmount * winningQuotation.InterestRate) / float64(100)
		installmentDetail.TotalEMI = installmentDetail.PrincipalAmount + installmentDetail.InterestAmount
		installmentDetail.RepaymentStatus = STATE_DEMANDED
		repaymentSchedule = append(repaymentSchedule, installmentDetail)
	}

	return repaymentSchedule
}

func (t *SmartLendingChaincode) CheckLoanDefaultStatus(applicationDetails LoanApplication) LoanApplication {

	// Loop through the repayment schedule to mark the loan default status accordingly
	var countOfMissedPayments int = 0
	for i := 0; i < len(applicationDetails.RepaymentSchedule); i++ {
		if applicationDetails.RepaymentSchedule[i].RepaymentStatus == STATE_MISSED {
			countOfMissedPayments++
		}
	}

	// Mark the loan as default if missed payments are more than 3
	if countOfMissedPayments >= 3 {
		applicationDetails.Status = STATE_NON_PERFORMING
	} else {
		applicationDetails.Status = STATE_PERFORMING
	}

	return applicationDetails
}

//==============================================================================================================================
//	 Main
//==============================================================================================================================

func main() {
	err := shim.Start(new(SmartLendingChaincode))
	if err != nil {
		fmt.Println("Could not start SmartLendingChaincode" + err.Error())
	} else {
		fmt.Println("SmartLendingChaincode successfully started")
	}

}
