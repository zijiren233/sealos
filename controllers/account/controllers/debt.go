package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	runtime2 "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	client2 "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	v1 "github.com/labring/sealos/controllers/account/api/v1"
	utils2 "github.com/labring/sealos/controllers/account/controllers/utils"
	"github.com/labring/sealos/controllers/pkg/database"
	"github.com/labring/sealos/controllers/pkg/database/cockroach"
	"github.com/labring/sealos/controllers/pkg/types"
	"github.com/labring/sealos/controllers/pkg/utils"
	"github.com/labring/sealos/controllers/pkg/utils/env"
	dlock "github.com/labring/sealos/controllers/pkg/utils/lock"
	"github.com/labring/sealos/controllers/pkg/utils/maps"
	userv1 "github.com/labring/sealos/controllers/user/api/v1"
	"github.com/volcengine/volc-sdk-golang/service/vms"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	DebtDetectionCycleEnv = "DebtDetectionCycleSeconds"

	finalDeletionDebtNamespaceSyncInterval = time.Hour
	finalDeletionDebtNamespaceQueryTimeout = 30 * time.Second
	flushDebtResourceStatusRequestTimeout  = 30 * time.Second

	SMSAccessKeyIDEnv     = "SMS_AK"
	SMSAccessKeySecretEnv = "SMS_SK"
	VmsAccessKeyIDEnv     = "VMS_AK"
	VmsAccessKeySecretEnv = "VMS_SK"
	SMSEndpointEnv        = "SMS_ENDPOINT"
	SMSSignNameEnv        = "SMS_SIGN_NAME"
	SMSCodeMapEnv         = "SMS_CODE_MAP"
	VmsCodeMapEnv         = "VMS_CODE_MAP"
	VmsNumberPollEnv      = "VMS_NUMBER_POLL"
	SMTPHostEnv           = "SMTP_HOST"
	SMTPPortEnv           = "SMTP_PORT"
	SMTPFromEnv           = "SMTP_FROM"
	SMTPUserEnv           = "SMTP_USER"
	SMTPPasswordEnv       = "SMTP_PASSWORD"
	SMTPTitleEnv          = "SMTP_TITLE"
)

// DebtReconciler reconciles a Debt object
type DebtReconciler struct {
	client.Client
	*AccountReconciler
	AccountV2           database.AccountV2
	InitUserAccountFunc func(user *types.UserQueryOpts) (*types.Account, error)
	Scheme              *runtime.Scheme
	DebtDetectionCycle  time.Duration
	LocalRegionID       string
	logr.Logger
	accountSystemNamespace      string
	SmsConfig                   *SmsConfig
	VmsConfig                   *VmsConfig
	smtpConfig                  *utils2.SMTPConfig
	DebtUserMap                 *maps.ConcurrentMap
	processID                   string
	SkipExpiredUserTimeDuration time.Duration
	SendDebtStatusEmailBody     map[v1.DebtStatusType]string
	debtEmailLanguage           string
}

type VmsConfig struct {
	TemplateCode map[string]string
	NumberPoll   string
}

type SmsConfig struct {
	Client      *client2.Client
	SmsSignName string
	SmsCode     map[string]string
}

var DebtConfig = v1.DefaultDebtConfig

func (r *DebtReconciler) DetermineCurrentStatus(
	oweamount int64,
	_ uuid.UUID,
	updateIntervalSeconds int64,
	lastStatus v1.DebtStatusType,
) (v1.DebtStatusType, error) {
	return determineCurrentStatus(oweamount, updateIntervalSeconds, lastStatus), nil
}

func determineCurrentStatus(
	oweamount, updateIntervalSeconds int64,
	lastStatus v1.DebtStatusType,
) v1.DebtStatusType {
	if oweamount > 0 {
		if oweamount > 10*BaseUnit {
			return v1.NormalPeriod
		} else if oweamount > 5*BaseUnit {
			return v1.LowBalancePeriod
		}
		return v1.CriticalBalancePeriod
	}
	if lastStatus == v1.NormalPeriod || lastStatus == v1.LowBalancePeriod ||
		lastStatus == v1.CriticalBalancePeriod {
		return v1.DebtPeriod
	}
	if lastStatus == v1.DebtPeriod && updateIntervalSeconds >= DebtConfig[v1.DebtDeletionPeriod] {
		return v1.DebtDeletionPeriod
	}
	if lastStatus == v1.DebtDeletionPeriod &&
		updateIntervalSeconds >= DebtConfig[v1.FinalDeletionPeriod] {
		return v1.FinalDeletionPeriod
	}
	return lastStatus // Maintain current debt state if no transition
}

const (
	// fromEn = "Debt-System"
	// fromZh = "欠费系统"
	// languageEn = "en"
	// languageZh       = "zh"
	// debtChoicePrefix = "debt-choice-"
	// readStatusLabel  = "isRead"
	// falseStatus      = "false"
	trueStatus = "true"
)

var (
	forbidTimes = []string{"00:00-10:00", "20:00-24:00"}
	UTCPlus8    = time.FixedZone("UTC+8", 8*3600)
)

// GetSendVmsTimeInUTCPlus8 send vms time in UTC+8 10:00-20:00
func GetSendVmsTimeInUTCPlus8(t time.Time) time.Time {
	nowInUTCPlus8 := t.In(UTCPlus8)
	hour := nowInUTCPlus8.Hour()
	if hour >= 10 && hour < 20 {
		return t
	}
	var next10AM time.Time
	if hour < 10 {
		next10AM = time.Date(
			nowInUTCPlus8.Year(),
			nowInUTCPlus8.Month(),
			nowInUTCPlus8.Day(),
			10,
			0,
			0,
			0,
			UTCPlus8,
		)
	} else {
		next10AM = time.Date(
			nowInUTCPlus8.Year(),
			nowInUTCPlus8.Month(),
			nowInUTCPlus8.Day()+1,
			10,
			0,
			0,
			0,
			UTCPlus8,
		)
	}
	return next10AM.In(time.Local)
}

// convert "1:code1,2:code2" to map[int]string
func splitSmsCodeMap(codeStr string) (map[string]string, error) {
	codeMap := make(map[string]string)
	for code := range strings.SplitSeq(codeStr, ",") {
		split := strings.SplitN(code, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("invalid sms code map: %s", codeStr)
		}
		codeMap[split[0]] = split[1]
	}
	return codeMap, nil
}

func (r *DebtReconciler) setupSmsConfig() error {
	if err := env.CheckEnvSetting(
		[]string{
			SMSAccessKeyIDEnv,
			SMSAccessKeySecretEnv,
			SMSEndpointEnv,
			SMSSignNameEnv,
			SMSCodeMapEnv,
		},
	); err != nil {
		return fmt.Errorf("check env setting error: %w", err)
	}

	smsCodeMap, err := splitSmsCodeMap(os.Getenv(SMSCodeMapEnv))
	if err != nil {
		return fmt.Errorf("split sms code map error: %w", err)
	}
	for key := range smsCodeMap {
		if _, ok := types.StatusMap[types.DebtStatusType(key)]; !ok {
			return fmt.Errorf("invalid sms code map key: %s", key)
		}
	}
	r.Info("set sms code map", "smsCodeMap", smsCodeMap, "smsSignName", os.Getenv(SMSSignNameEnv))
	smsClient, err := utils2.CreateSMSClient(
		os.Getenv(SMSAccessKeyIDEnv),
		os.Getenv(SMSAccessKeySecretEnv),
		os.Getenv(SMSEndpointEnv),
	)
	if err != nil {
		return fmt.Errorf("create sms client error: %w", err)
	}
	r.SmsConfig = &SmsConfig{
		Client:      smsClient,
		SmsSignName: os.Getenv(SMSSignNameEnv),
		SmsCode:     smsCodeMap,
	}
	return nil
}

func (r *DebtReconciler) setupVmsConfig() error {
	if err := env.CheckEnvSetting(
		[]string{VmsAccessKeyIDEnv, VmsAccessKeySecretEnv, VmsNumberPollEnv},
	); err != nil {
		return fmt.Errorf("check env setting error: %w", err)
	}
	vms.DefaultInstance.SetAccessKey(os.Getenv(VmsAccessKeyIDEnv))
	vms.DefaultInstance.SetSecretKey(os.Getenv(VmsAccessKeySecretEnv))

	vmsCodeMap, err := splitSmsCodeMap(os.Getenv(VmsCodeMapEnv))
	if err != nil {
		return fmt.Errorf("split vms code map error: %w", err)
	}
	for key := range vmsCodeMap {
		if _, ok := types.StatusMap[types.DebtStatusType(key)]; !ok {
			return fmt.Errorf("invalid sms code map key: %s", key)
		}
	}
	r.Info("set vms code map", "vmsCodeMap", vmsCodeMap)
	r.VmsConfig = &VmsConfig{
		TemplateCode: vmsCodeMap,
		NumberPoll:   os.Getenv(VmsNumberPollEnv),
	}
	return nil
}

func (r *DebtReconciler) setupSMTPConfig() error {
	if err := env.CheckEnvSetting(
		[]string{SMTPHostEnv, SMTPFromEnv, SMTPPasswordEnv, SMTPTitleEnv},
	); err != nil {
		return fmt.Errorf("check env setting error: %w", err)
	}
	serverPort, err := strconv.Atoi(env.GetEnvWithDefault(SMTPPortEnv, "465"))
	if err != nil {
		return fmt.Errorf("invalid smtp port: %w", err)
	}
	r.smtpConfig = &utils2.SMTPConfig{
		ServerHost: os.Getenv(SMTPHostEnv),
		ServerPort: serverPort,
		Username:   env.GetEnvWithDefault(SMTPUserEnv, os.Getenv(SMTPFromEnv)),
		FromEmail:  os.Getenv(SMTPFromEnv),
		Passwd:     os.Getenv(SMTPPasswordEnv),
		EmailTitle: os.Getenv(SMTPTitleEnv),
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DebtReconciler) SetupWithManager(mgr ctrl.Manager, rateOpts controller.Options) error {
	r.Init()
	/*
		{"DebtConfig":{
		"ApproachingDeletionPeriod":345600,
		"FinalDeletionPeriod":604800,
		"ImminentDeletionPeriod":259200,"WarningPeriod":0},
		"DebtDetectionCycle": "1m0s",
		"accountSystemNamespace": "account-system",
		"accountNamespace": "sealos-system"}
	*/
	r.Info("set config", "DebtConfig", DebtConfig, "DebtDetectionCycle", r.DebtDetectionCycle,
		"accountSystemNamespace", r.accountSystemNamespace)
	return ctrl.NewControllerManagedBy(mgr).
		For(&userv1.User{}, builder.WithPredicates(predicate.And(UserOwnerPredicate{})), builder.OnlyMetadata).
		Watches(&v1.Payment{}, &handler.EnqueueRequestForObject{}).
		WithOptions(rateOpts).
		Complete(r)
}

func (r *DebtReconciler) Init() {
	r.Logger = ctrl.Log.WithName("DebtController")
	r.accountSystemNamespace = env.GetEnvWithDefault(v1.AccountSystemNamespaceEnv, "account-system")
	r.LocalRegionID = os.Getenv(cockroach.EnvLocalRegion)
	debtDetectionCycleSecond := env.GetInt64EnvWithDefault(DebtDetectionCycleEnv, 1800)
	r.DebtDetectionCycle = time.Duration(debtDetectionCycleSecond) * time.Second
	r.processID = uuid.NewString()

	currency := strings.ToLower(strings.TrimSpace(os.Getenv("STRIPE_CURRENCY")))
	if currency == "usd" {
		r.debtEmailLanguage = "en"
	} else {
		r.debtEmailLanguage = "zh"
	}

	setupList := []func() error{
		r.setupSmsConfig,
		r.setupVmsConfig,
		r.setupSMTPConfig,
	}
	for i := range setupList {
		if err := setupList[i](); err != nil {
			r.Error(
				err,
				"failed to set up "+runtime2.FuncForPC(reflect.ValueOf(setupList[i]).Pointer()).
					Name(),
			)
		}
	}
	setDefaultDebtPeriodWaitSecond()
	r.SendDebtStatusEmailBody = make(map[v1.DebtStatusType]string)
	for _, status := range []v1.DebtStatusType{
		v1.LowBalancePeriod,
		v1.CriticalBalancePeriod,
		v1.DebtPeriod,
		v1.DebtDeletionPeriod,
		v1.FinalDeletionPeriod,
	} {
		if body := os.Getenv(string(status) + "EmailBody"); body != "" {
			r.SendDebtStatusEmailBody[status] = body
		}
	}
	r.Info(
		"debt config",
		"DebtConfig",
		DebtConfig,
		"DebtDetectionCycle",
		r.DebtDetectionCycle,
		"debtEmailLanguage",
		r.debtEmailLanguage,
	)
}

func setDefaultDebtPeriodWaitSecond() {
	DebtConfig[v1.DebtDeletionPeriod] = env.GetInt64EnvWithDefault(
		string(v1.DebtDeletionPeriod),
		7*v1.DaySecond,
	)
	DebtConfig[v1.FinalDeletionPeriod] = env.GetInt64EnvWithDefault(
		string(v1.FinalDeletionPeriod),
		7*v1.DaySecond,
	)
}

type UserOwnerPredicate struct {
	predicate.Funcs
}

func (UserOwnerPredicate) Create(e event.CreateEvent) bool {
	owner := e.Object.GetAnnotations()[userv1.UserAnnotationOwnerKey]
	return owner != "" && owner == e.Object.GetName()
}

func (UserOwnerPredicate) Update(_ event.UpdateEvent) bool {
	return false
}

func (r *DebtReconciler) Start(ctx context.Context) error {
	lock := dlock.NewDistributedLock(r.AccountV2.GetGlobalDB(), "debt_reconciler", r.processID)
	for {
		err := lock.TryLock(ctx, 15*time.Second)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return nil
		}
		if !errors.Is(err, dlock.ErrLockNotAcquired) {
			return fmt.Errorf("acquire debt reconciler lock: %w", err)
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			log.Printf("failed to unlock: %v", err)
		}
	}()
	log.Printf("debt reconciler lock acquired, process ID: %s", r.processID)
	r.start(ctx)
	log.Printf("debt reconciler stopped")
	return nil
}

func (r *DebtReconciler) start(ctx context.Context) {
	db := r.AccountV2.GetGlobalDB()
	var wg sync.WaitGroup

	// 1.1 account update processing
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.processWithTimeRange(
			ctx,
			&types.Account{},
			"updated_at",
			1*time.Minute,
			24*time.Hour,
			func(db *gorm.DB, start, end time.Time) error {
				users, err := getUniqueUsers(db, &types.Account{}, "updated_at", start, end)
				if err != nil {
					return fmt.Errorf("failed to get unique users: %w", err)
				}
				if len(users) > 0 {
					r.Info(
						"processed account updates",
						"count",
						len(users),
						"start",
						start,
						"end",
						end,
					)
					if err := r.EnqueueBalanceAlertUsers(users, "account-update"); err != nil {
						return err
					}
				}
				return nil
			},
		)
	}()

	// 1.2 the arrears are transferred to the clearing state
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			var users []uuid.UUID
			transitionBefore := time.Now().UTC().Add(
				-time.Duration(DebtConfig[v1.DebtDeletionPeriod]) * time.Second,
			)
			if err := db.Model(&types.Debt{}).
				Where("account_debt_status = ? AND updated_at < ?", types.DebtPeriod, transitionBefore).
				Distinct("user_uid").
				Pluck("user_uid", &users).
				Error; err != nil {
				r.Error(
					err,
					"failed to query unique users",
					"account_debt_status",
					types.DebtPeriod,
					"updated_at",
					transitionBefore,
				)
				continue
			}
			if len(users) > 0 {
				if err := r.EnqueueBalanceAlertUsers(users, "debt-grace-period"); err != nil {
					r.Error(err, "failed to enqueue debt grace-period users")
					continue
				}
				r.Info(
					"processed debt status",
					"count",
					len(users),
					"updated_atBefore",
					transitionBefore,
				)
			}
		}
	}()

	// 1.3 clearing changes to delete state
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			var users []uuid.UUID
			transitionBefore := time.Now().UTC().Add(
				-time.Duration(DebtConfig[v1.FinalDeletionPeriod]) * time.Second,
			)
			if err := db.Model(&types.Debt{}).
				Where("account_debt_status = ? AND updated_at < ?", types.DebtDeletionPeriod, transitionBefore).
				Distinct("user_uid").
				Pluck("user_uid", &users).
				Error; err != nil {
				r.Error(
					err,
					"failed to query unique users",
					"account_debt_status",
					types.DebtDeletionPeriod,
					"updated_at",
					transitionBefore,
				)
				continue
			}
			if len(users) > 0 {
				if err := r.EnqueueBalanceAlertUsers(users, "debt-deletion-period"); err != nil {
					r.Error(err, "failed to enqueue debt deletion-period users")
					continue
				}
				r.Info(
					"processed debt status",
					"count",
					len(users),
					"updated_atBefore",
					transitionBefore,
				)
			}
		}
	}()

	// 1.4 replay final deletion status for accounts that are already in the terminal debt state.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.syncFinalDeletionDebtNamespacesLoop(ctx, db)
	}()

	// 2 credits issue, consumption, and status changes.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.processWithTimeRange(
			ctx,
			&types.Credits{},
			"updated_at",
			1*time.Minute,
			24*time.Hour,
			func(db *gorm.DB, start, end time.Time) error {
				users, err := getUniqueUsers(db, &types.Credits{}, "updated_at", start, end)
				if err != nil {
					return fmt.Errorf("failed to get credits users: %w", err)
				}
				return r.EnqueueBalanceAlertUsers(users, "credits-update")
			},
		)
	}()

	// 3 process the coalescing prediction queue.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.processBalanceAlertQueueLoop(ctx)
	}()

	// 4 enqueue active users and expiring credits as an hourly safety net.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.enqueueBalanceAlertCompensationLoop(ctx)
	}()

	// 5 deliver independently retryable notification outbox entries.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.dispatchBalanceAlertDeliveriesLoop(ctx)
	}()

	wg.Wait()
}

func (r *DebtReconciler) syncFinalDeletionDebtNamespacesLoop(ctx context.Context, db *gorm.DB) {
	run := func() {
		if err := r.syncFinalDeletionDebtNamespaces(ctx, db); err != nil {
			r.Error(err, "failed to sync final deletion debt namespaces")
		}
	}

	run()
	ticker := time.NewTicker(finalDeletionDebtNamespaceSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (r *DebtReconciler) syncFinalDeletionDebtNamespaces(ctx context.Context, db *gorm.DB) error {
	var users []uuid.UUID
	queryCtx, cancel := context.WithTimeout(ctx, finalDeletionDebtNamespaceQueryTimeout)
	defer cancel()

	if err := db.WithContext(queryCtx).Model(&types.Debt{}).
		Where("account_debt_status = ?", types.FinalDeletionPeriod).
		Distinct("user_uid").
		Pluck("user_uid", &users).Error; err != nil {
		return fmt.Errorf("failed to query final deletion debt users: %w", err)
	}

	var errs []error
	for _, userUID := range users {
		if err := r.syncFinalDeletionDebtNamespacesForUser(ctx, userUID); err != nil {
			errMsg := "failed to sync final deletion debt namespaces " +
				"for user %s: %w"
			errs = append(errs, fmt.Errorf(errMsg, userUID, err))
		}
	}
	if len(users) > 0 {
		r.Info("synced final deletion debt namespaces", "users", len(users))
	}
	return errors.Join(errs...)
}

func (r *DebtReconciler) syncFinalDeletionDebtNamespacesForUser(
	ctx context.Context,
	userUID uuid.UUID,
) error {
	return r.sendFlushDebtResourceStatusRequestWithContext(
		ctx,
		finalDeletionDebtNamespaceFlushReq(userUID),
	)
}

func finalDeletionDebtNamespaceFlushReq(userUID uuid.UUID) AdminFlushResourceStatusReq {
	return AdminFlushResourceStatusReq{
		UserUID:           userUID,
		LastDebtStatus:    types.DebtDeletionPeriod,
		CurrentDebtStatus: types.FinalDeletionPeriod,
	}
}

func (r *DebtReconciler) RefreshDebtStatus(userUID uuid.UUID) error {
	account, err := r.AccountV2.GetAccountWithCredits(userUID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to get account %s: %w", userUID, err)
	}
	if account == nil {
		return fmt.Errorf("account %s not found", userUID)
	}
	debt := types.Debt{}
	err = r.AccountV2.GetGlobalDB().
		Model(&types.Debt{}).
		Where("user_uid = ?", userUID).
		First(&debt).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to get debt %s: %w", userUID, err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	lastStatus := debt.AccountDebtStatus
	update := false
	if lastStatus == "" {
		lastStatus = types.NormalPeriod
		update = true
	}
	isBasicUser := account.Balance <= 10*BaseUnit
	now := time.Now().UTC()
	prediction, err := r.loadBalancePrediction(userUID, account, now)
	if err != nil {
		r.Error(err, "failed to load balance prediction; using fixed balance thresholds", "userUID", userUID)
		prediction = balancePrediction{
			AvailableBalance: account.Balance - account.DeductionBalance + account.UsableCredits,
			Confidence:       types.BalancePredictionConfidenceLow,
			ConfidenceReason: "prediction data could not be loaded",
		}
	}
	statusAgeSeconds := now.Unix() - debt.UpdatedAt.UTC().Unix()
	assessment := assessBalanceRisk(prediction, lastStatus, statusAgeSeconds)
	currentStatus, err := r.effectiveBalanceStatus(userUID, lastStatus, assessment)
	if err != nil {
		return fmt.Errorf("determine effective balance status for user %s: %w", userUID, err)
	}
	if lastStatus != currentStatus {
		if err := r.sendFlushDebtResourceStatusRequest(AdminFlushResourceStatusReq{
			UserUID:           userUID,
			LastDebtStatus:    lastStatus,
			CurrentDebtStatus: currentStatus,
			IsBasicUser:       isBasicUser,
		}); err != nil {
			return fmt.Errorf("failed to send flush resource status request: %w", err)
		}
	}

	if lastStatus != currentStatus && types.ContainDebtStatus(types.DebtStates, currentStatus) {
		if err = r.ResumeBalance(userUID); err != nil {
			return fmt.Errorf("failed to normalize overdrawn balance: %w", err)
		}
	}

	userID, userName := "", ""
	var deliverySpecs []balanceAlertDeliverySpec
	if assessment.Risk {
		skipBasicLow := isBasicUser &&
			assessment.Status == types.LowBalancePeriod &&
			prediction.Confidence == types.BalancePredictionConfidenceLow
		userID, userName, deliverySpecs, err = r.balanceAlertAudience(
			userUID,
			assessment.Status,
			skipBasicLow,
		)
		if err != nil {
			return fmt.Errorf("load balance alert audience: %w", err)
		}
	}
	var etaSeconds *int64
	if prediction.ETA != nil {
		seconds := int64(prediction.ETA.Seconds())
		etaSeconds = &seconds
	}
	payload := balanceAlertPayload{
		UserUID: userUID, UserID: userID, UserName: userName,
		AlertLevel: assessment.Status, AvailableBalance: prediction.AvailableBalance,
		ETASeconds: etaSeconds, ExhaustedAt: prediction.ExhaustedAt,
		LongHourlyRate: prediction.LongRate, ShortHourlyRate: prediction.ShortRate,
		ForecastRate: prediction.ForecastRate, TopWorkspace: prediction.TopWorkspace,
		TopApplication: prediction.TopApplication, Confidence: prediction.Confidence,
		ConfidenceReason: prediction.ConfidenceReason, DataThrough: prediction.DataThrough,
		Domain: r.AccountV2.GetLocalRegion().Domain, Language: r.debtEmailLanguage,
	}

	statusChanged := lastStatus != currentStatus || update
	err = r.AccountV2.GlobalTransactionHandler(func(tx *gorm.DB) error {
		if err := r.applyBalanceAlertEpisode(
			tx, userUID, assessment, prediction, payload, deliverySpecs, now,
		); err != nil {
			return err
		}
		if !statusChanged {
			return nil
		}
		debt.AccountDebtStatus = currentStatus
		debt.UpdatedAt = now
		if dErr := tx.Model(&types.Debt{}).Where("user_uid = ?", userUID).Save(debt).Error; dErr != nil {
			return fmt.Errorf("failed to save debt: %w", dErr)
		}
		debtRecord := types.DebtStatusRecord{
			ID: uuid.New(), UserUID: userUID, LastStatus: lastStatus,
			CurrentStatus: currentStatus, CreateAt: now,
		}
		if sErr := tx.Model(&types.DebtStatusRecord{}).Create(&debtRecord).Error; sErr != nil {
			return fmt.Errorf("failed to save debt status record: %w", sErr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to save debt status: %w", err)
	}
	if statusChanged {
		r.Logger.Info(
			"updated predicted balance status",
			"userUID", userUID,
			"lastStatus", lastStatus,
			"currentStatus", currentStatus,
			"confidence", prediction.Confidence,
			"dataThrough", prediction.DataThrough,
		)
	}
	return nil
}

func (r *DebtReconciler) ResumeBalance(userUID uuid.UUID) error {
	account, err := r.AccountV2.GetAccount(&types.UserQueryOpts{UID: userUID})
	if err != nil {
		return fmt.Errorf("failed to get account %s: %w", userUID, err)
	}
	if account.DeductionBalance <= account.Balance {
		return nil
	}
	err = r.AccountV2.GlobalTransactionHandler(func(tx *gorm.DB) error {
		result := tx.Model(&types.Account{}).
			Where(`"userUid" = ?`, userUID).
			Where(`"deduction_balance" > "balance"`).
			Updates(map[string]any{
				"deduction_balance": gorm.Expr("balance"),
			})
		if result.Error != nil {
			return fmt.Errorf("failed to update account balance: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			return tx.Create(&types.DebtResumeDeductionBalanceTransaction{
				UserUID:                userUID,
				BeforeDeductionBalance: account.DeductionBalance,
				AfterDeductionBalance:  account.Balance,
				BeforeBalance:          account.Balance,
			}).Error
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update account balance: %w", err)
	}
	return nil
}

type AdminFlushResourceStatusReq struct {
	UserUID           uuid.UUID            `json:"userUID"           bson:"userUID"`
	LastDebtStatus    types.DebtStatusType `json:"lastDebtStatus"    bson:"lastDebtStatus"`
	CurrentDebtStatus types.DebtStatusType `json:"currentDebtStatus" bson:"currentDebtStatus"`
	IsBasicUser       bool                 `json:"isBasicUser"       bson:"isBasicUser"`
}

// TODO flush desktop message (send or read) && flush resource quota (suspend or resume or delete)
func (r *DebtReconciler) sendFlushDebtResourceStatusRequest(
	quotaReq AdminFlushResourceStatusReq,
) error {
	return r.sendFlushDebtResourceStatusRequestWithContext(context.Background(), quotaReq)
}

func (r *DebtReconciler) sendFlushDebtResourceStatusRequestWithContext(
	ctx context.Context,
	quotaReq AdminFlushResourceStatusReq,
) error {
	client := http.Client{
		Timeout: flushDebtResourceStatusRequestTimeout,
	}

	for _, domain := range r.allRegionDomain {
		token, err := r.jwtManager.GenerateToken(utils.JwtUser{
			Requester: AdminUserName,
		})
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		prefix := "https://"
		if strings.Contains(domain, "nip.io") {
			prefix = "http://"
		}
		url := fmt.Sprintf(
			prefix+"account-api.%s/admin/v1alpha1/flush-debt-resource-status",
			domain,
		)

		quotaReqBody, err := json.Marshal(quotaReq)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		var lastErr error
		backoffTime := time.Second

		maxRetries := 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				url,
				bytes.NewBuffer(quotaReqBody),
			)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("failed to send request: %w", err)
			} else {
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					lastErr = nil
					break
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					lastErr = fmt.Errorf(
						"unexpected status code: %d, failed to read response body: %w",
						resp.StatusCode,
						err,
					)
				} else {
					lastErr = fmt.Errorf(
						"unexpected status code: %d, response body: %s",
						resp.StatusCode,
						string(body),
					)
				}
			}

			// 进行重试
			if attempt < maxRetries {
				fmt.Printf(
					"Attempt %d failed: %v. Retrying in %v...\n",
					attempt,
					lastErr,
					backoffTime,
				)
				time.Sleep(backoffTime)
				backoffTime *= 2 // 指数增长退避时间
			}
		}
		if lastErr != nil {
			return fmt.Errorf(
				"failed to send %s request after %d attempts: %w",
				url,
				maxRetries,
				lastErr,
			)
		}
	}
	return nil
}

// 获取时间范围内的不重复用户 UUID
func getUniqueUsers(
	db *gorm.DB,
	table any,
	timeField string,
	startTime, endTime time.Time,
) ([]uuid.UUID, error) {
	var users []uuid.UUID
	switch table.(type) {
	case *types.Account:
		if err := db.Model(table).Where(timeField+" BETWEEN ? AND ?", startTime, endTime).
			// Where("deduction_balance > ?", 0).
			Distinct(`"userUid"`).Pluck(`"userUid"`, &users).Error; err != nil {
			return nil, fmt.Errorf("failed to query unique users: %w", err)
		}
	case *types.AccountTransaction, *types.Payment:
		if err := db.Model(table).Where(timeField+" BETWEEN ? AND ?", startTime, endTime).
			Distinct(`"userUid"`).Pluck(`"userUid"`, &users).Error; err != nil {
			return nil, fmt.Errorf("failed to query unique users: %w", err)
		}
	default:
		if err := db.Model(table).Where(timeField+" BETWEEN ? AND ?", startTime, endTime).
			Distinct("user_uid").Pluck("user_uid", &users).Error; err != nil {
			return nil, fmt.Errorf("failed to query unique users: %w", err)
		}
	}
	return users, nil
}

// 时间区间轮询处理
func (r *DebtReconciler) processWithTimeRange(
	ctx context.Context,
	table any,
	timeField string,
	interval, initialDuration time.Duration,
	processFunc func(*gorm.DB, time.Time, time.Time) error,
) {
	// 首次处理
	startTime := time.Now().Add(-initialDuration)
	endTime := time.Now().Add(-2 * time.Minute)
	if err := processFunc(r.AccountV2.GetGlobalDB(), startTime, endTime); err != nil {
		r.Error(
			err,
			"failed to process initial time range",
			"table",
			fmt.Sprintf("%T", table),
			"start",
			startTime,
			"end",
			endTime,
		)
		endTime = startTime
	}

	// 后续按时间区间轮询
	lastEndTime := endTime
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		startTime = lastEndTime
		endTime = time.Now().Add(-interval)
		// if error occurs, the start time of the next execution is the start time of the last one
		if err := processFunc(r.AccountV2.GetGlobalDB(), startTime, endTime); err != nil {
			r.Error(
				err,
				"failed to process time range",
				"start",
				startTime,
				"end",
				endTime,
				"table",
				fmt.Sprintf("%T", table),
			)
			continue
		}
		lastEndTime = endTime
	}
}
