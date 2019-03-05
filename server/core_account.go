// Copyright 2018 The Nakama Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"database/sql"
	"github.com/cockroachdb/cockroach-go/crdb"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/heroiclabs/nakama/api"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var ErrAccountNotFound = errors.New("account not found")

func GetAccount(ctx context.Context, logger *zap.Logger, db *sql.DB, tracker Tracker, userID uuid.UUID) (*api.Account, error) {
	var displayName sql.NullString
	var username sql.NullString
	var avatarURL sql.NullString
	var langTag sql.NullString
	var location sql.NullString
	var timezone sql.NullString
	var metadata sql.NullString
	var wallet sql.NullString
	var email sql.NullString
	var facebook sql.NullString
	var google sql.NullString
	var gamecenter sql.NullString
	var steam sql.NullString
	var customID sql.NullString
	var edgeCount int
	var createTime pq.NullTime
	var updateTime pq.NullTime
	var verifyTime pq.NullTime
	var deviceIDs pq.StringArray

	query := `
SELECT u.username, u.display_name, u.avatar_url, u.lang_tag, u.location, u.timezone, u.metadata, u.wallet,
	u.email, u.facebook_id, u.google_id, u.gamecenter_id, u.steam_id, u.custom_id, u.edge_count,
	u.create_time, u.update_time, u.verify_time, array(select ud.id from user_device ud where u.id = ud.user_id)
FROM users u
WHERE u.id = $1`

	if err := db.QueryRowContext(ctx, query, userID).Scan(&username, &displayName, &avatarURL, &langTag, &location, &timezone, &metadata, &wallet, &email, &facebook, &google, &gamecenter, &steam, &customID, &edgeCount, &createTime, &updateTime, &verifyTime, &deviceIDs); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAccountNotFound
		}
		logger.Error("Error retrieving user account.", zap.Error(err))
		return nil, err
	}

	devices := make([]*api.AccountDevice, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		devices = append(devices, &api.AccountDevice{Id: deviceID})
	}

	var verifyTimestamp *timestamp.Timestamp = nil
	if verifyTime.Valid && verifyTime.Time.Unix() != 0 {
		verifyTimestamp = &timestamp.Timestamp{Seconds: verifyTime.Time.Unix()}
	}

	online := false
	if tracker != nil {
		online = tracker.StreamExists(PresenceStream{Mode: StreamModeNotifications, Subject: userID})
	}

	return &api.Account{
		User: &api.User{
			Id:           userID.String(),
			Username:     username.String,
			DisplayName:  displayName.String,
			AvatarUrl:    avatarURL.String,
			LangTag:      langTag.String,
			Location:     location.String,
			Timezone:     timezone.String,
			Metadata:     metadata.String,
			FacebookId:   facebook.String,
			GoogleId:     google.String,
			GamecenterId: gamecenter.String,
			SteamId:      steam.String,
			EdgeCount:    int32(edgeCount),
			CreateTime:   &timestamp.Timestamp{Seconds: createTime.Time.Unix()},
			UpdateTime:   &timestamp.Timestamp{Seconds: updateTime.Time.Unix()},
			Online:       online,
		},
		Wallet:     wallet.String,
		Email:      email.String,
		Devices:    devices,
		CustomId:   customID.String,
		VerifyTime: verifyTimestamp,
	}, nil
}

func GetAccounts(ctx context.Context, logger *zap.Logger, db *sql.DB, tracker Tracker, userIDs []string) ([]*api.Account, error) {
	statements := make([]string, 0, len(userIDs))
	parameters := make([]interface{}, 0, len(userIDs))
	for _, userID := range userIDs {
		parameters = append(parameters, userID)
		statements = append(statements, "$"+strconv.Itoa(len(parameters)))
	}

	query := `
SELECT u.id, u.username, u.display_name, u.avatar_url, u.lang_tag, u.location, u.timezone, u.metadata, u.wallet,
	u.email, u.facebook_id, u.google_id, u.gamecenter_id, u.steam_id, u.custom_id, u.edge_count,
	u.create_time, u.update_time, u.verify_time, array(select ud.id from user_device ud where u.id = ud.user_id)
FROM users u
WHERE u.id IN (` + strings.Join(statements, ",") + `)`
	rows, err := db.QueryContext(ctx, query, parameters...)
	if err != nil {
		logger.Error("Error retrieving user accounts.", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	accounts := make([]*api.Account, 0, len(userIDs))
	for rows.Next() {
		var userID string
		var username sql.NullString
		var displayName sql.NullString
		var avatarURL sql.NullString
		var langTag sql.NullString
		var location sql.NullString
		var timezone sql.NullString
		var metadata sql.NullString
		var wallet sql.NullString
		var email sql.NullString
		var facebook sql.NullString
		var google sql.NullString
		var gamecenter sql.NullString
		var steam sql.NullString
		var customID sql.NullString
		var edgeCount int
		var createTime pq.NullTime
		var updateTime pq.NullTime
		var verifyTime pq.NullTime
		var deviceIDs pq.StringArray

		err = rows.Scan(&userID, &username, &displayName, &avatarURL, &langTag, &location, &timezone, &metadata, &wallet, &email, &facebook, &google, &gamecenter, &steam, &customID, &edgeCount, &createTime, &updateTime, &verifyTime, &deviceIDs)
		if err != nil {
			logger.Error("Error retrieving user accounts.", zap.Error(err))
			return nil, err
		}

		devices := make([]*api.AccountDevice, 0, len(deviceIDs))
		for _, deviceID := range deviceIDs {
			devices = append(devices, &api.AccountDevice{Id: deviceID})
		}

		var verifyTimestamp *timestamp.Timestamp
		if verifyTime.Valid && verifyTime.Time.Unix() != 0 {
			verifyTimestamp = &timestamp.Timestamp{Seconds: verifyTime.Time.Unix()}
		}

		online := false
		if tracker != nil {
			online = tracker.StreamExists(PresenceStream{Mode: StreamModeNotifications, Subject: uuid.FromStringOrNil(userID)})
		}

		accounts = append(accounts, &api.Account{
			User: &api.User{
				Id:           userID,
				Username:     username.String,
				DisplayName:  displayName.String,
				AvatarUrl:    avatarURL.String,
				LangTag:      langTag.String,
				Location:     location.String,
				Timezone:     timezone.String,
				Metadata:     metadata.String,
				FacebookId:   facebook.String,
				GoogleId:     google.String,
				GamecenterId: gamecenter.String,
				SteamId:      steam.String,
				EdgeCount:    int32(edgeCount),
				CreateTime:   &timestamp.Timestamp{Seconds: createTime.Time.Unix()},
				UpdateTime:   &timestamp.Timestamp{Seconds: updateTime.Time.Unix()},
				Online:       online,
			},
			Wallet:     wallet.String,
			Email:      email.String,
			Devices:    devices,
			CustomId:   customID.String,
			VerifyTime: verifyTimestamp,
		})
	}

	return accounts, nil
}

func UpdateAccount(ctx context.Context, logger *zap.Logger, db *sql.DB, userID uuid.UUID, username string, displayName, timezone, location, langTag, avatarURL, metadata *wrappers.StringValue) error {
	index := 1
	statements := make([]string, 0)
	params := make([]interface{}, 0)

	if username != "" {
		if invalidCharsRegex.MatchString(username) {
			return errors.New("Username invalid, no spaces or control characters allowed.")
		}
		statements = append(statements, "username = $"+strconv.Itoa(index))
		params = append(params, username)
		index++
	}

	if displayName != nil {
		if d := displayName.GetValue(); d == "" {
			statements = append(statements, "display_name = NULL")
		} else {
			statements = append(statements, "display_name = $"+strconv.Itoa(index))
			params = append(params, d)
			index++
		}
	}

	if timezone != nil {
		if t := timezone.GetValue(); t == "" {
			statements = append(statements, "timezone = NULL")
		} else {
			statements = append(statements, "timezone = $"+strconv.Itoa(index))
			params = append(params, t)
			index++
		}
	}

	if location != nil {
		if l := location.GetValue(); l == "" {
			statements = append(statements, "location = NULL")
		} else {
			statements = append(statements, "location = $"+strconv.Itoa(index))
			params = append(params, l)
			index++
		}
	}

	if langTag != nil {
		if l := langTag.GetValue(); l == "" {
			statements = append(statements, "lang_tag = NULL")
		} else {
			statements = append(statements, "lang_tag = $"+strconv.Itoa(index))
			params = append(params, l)
			index++
		}
	}

	if avatarURL != nil {
		if a := avatarURL.GetValue(); a == "" {
			statements = append(statements, "avatar_url = NULL")
		} else {
			statements = append(statements, "avatar_url = $"+strconv.Itoa(index))
			params = append(params, a)
			index++
		}
	}

	if metadata != nil {
		statements = append(statements, "metadata = $"+strconv.Itoa(index))
		params = append(params, metadata.GetValue())
		index++
	}

	if len(statements) == 0 {
		return errors.New("No fields to update.")
	}

	params = append(params, userID)

	query := "UPDATE users SET update_time = now(), " + strings.Join(statements, ", ") + " WHERE id = $" + strconv.Itoa(index)

	if _, err := db.ExecContext(ctx, query, params...); err != nil {
		if e, ok := err.(*pq.Error); ok && e.Code == dbErrorUniqueViolation && strings.Contains(e.Message, "users_username_key") {
			return errors.New("Username is already in use.")
		}

		logger.Error("Could not update user account.", zap.Error(err),
			zap.String("username", username),
			zap.Any("display_name", displayName.GetValue()),
			zap.Any("timezone", timezone.GetValue()),
			zap.Any("location", location.GetValue()),
			zap.Any("lang_tag", langTag.GetValue()),
			zap.Any("avatar_url", avatarURL.GetValue()))
		return err
	}

	return nil
}

func DeleteAccount(ctx context.Context, logger *zap.Logger, db *sql.DB, userID uuid.UUID, recorded bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("Could not begin database transaction.", zap.Error(err))
		return err
	}

	if err := crdb.ExecuteInTx(ctx, tx, func() error {
		count, err := DeleteUser(ctx, tx, userID)
		if err != nil {
			logger.Debug("Could not delete user", zap.Error(err), zap.String("user_id", userID.String()))
			return err
		} else if count == 0 {
			logger.Info("No user was found to delete. Skipping blacklist.", zap.String("user_id", userID.String()))
			return nil
		}

		err = LeaderboardRecordsDeleteAll(ctx, logger, tx, userID)
		if err != nil {
			logger.Debug("Could not delete leaderboard records.", zap.Error(err), zap.String("user_id", userID.String()))
			return err
		}

		err = GroupDeleteAll(ctx, logger, tx, userID)
		if err != nil {
			logger.Debug("Could not delete groups and relationships.", zap.Error(err), zap.String("user_id", userID.String()))
			return err
		}

		if recorded {
			_, err = tx.ExecContext(ctx, `INSERT INTO user_tombstone (user_id) VALUES ($1) ON CONFLICT(user_id) DO NOTHING`, userID)
			if err != nil {
				logger.Debug("Could not insert user ID into tombstone", zap.Error(err), zap.String("user_id", userID.String()))
				return err
			}
		}

		return nil
	}); err != nil {
		logger.Error("Error occurred while trying to delete the user.", zap.Error(err), zap.String("user_id", userID.String()))
		return err
	}

	return nil
}
