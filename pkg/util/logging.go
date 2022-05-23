/**
 * @license
 * Copyright 2020 Dynatrace LLC
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// Log is the shared Lumber Logger logging to console and after calling SetupLogging also to file
var Log *log.Logger

var requestLogFile afero.File
var responseLogFile afero.File

// SetupLogging is used to initialize the shared file Logger once the necessary setup config is available
func SetupLogging(fs afero.Fs, verbose bool) error {
	Log = log.New()
	Log.SetLevel(log.InfoLevel)
	log.SetOutput(os.Stdout)
	if verbose {
		Log.SetLevel(log.DebugLevel)
	}
	if _, err := fs.Stat(".logs"); os.IsNotExist(err) {
		err = fs.Mkdir(".logs", 0777)

		if err != nil {
			FailOnError(err, "could not create directory")
		}
	}
	logName := ".logs" + string(os.PathSeparator) + time.Now().Format("20060102-150405") + ".log"
	file, err := fs.OpenFile(logName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		Log.SetOutput(io.MultiWriter(os.Stderr, file))
	} else {
		Log.Info("Failed to log to file, using default stderr")
	}

	if err != nil {
		return err
	}
	err = setupRequestLog(fs)
	if err != nil {
		return err
	}
	err = setupResponseLog(fs)
	if err != nil {
		return err
	}
	return nil
}

func setupRequestLog(fs afero.Fs) error {
	if logFilePath, found := os.LookupEnv("MONACO_REQUEST_LOG"); found {
		logFilePath, err := filepath.Abs(logFilePath)

		if err != nil {
			return err
		}

		Log.Debug("request log activated at %s", logFilePath)
		handle, err := prepareLogFile(fs, logFilePath)

		if err != nil {
			return err
		}

		requestLogFile = handle
	} else {
		Log.Debug("request log not activated")
	}

	return nil
}

func setupResponseLog(fs afero.Fs) error {
	if logFilePath, found := os.LookupEnv("MONACO_RESPONSE_LOG"); found {
		logFilePath, err := filepath.Abs(logFilePath)

		if err != nil {
			return err
		}

		Log.Debug("response log activated at %s", logFilePath)
		handle, err := prepareLogFile(fs, logFilePath)

		if err != nil {
			return err
		}

		responseLogFile = handle
	} else {
		Log.Debug("response log not activated")
	}

	return nil
}

func prepareLogFile(fs afero.Fs, file string) (afero.File, error) {
	return fs.OpenFile(file, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
}

func IsRequestLoggingActive() bool {
	return requestLogFile != nil
}

func IsResponseLoggingActive() bool {
	return responseLogFile != nil
}

func LogRequest(id string, request *http.Request) error {
	if !IsRequestLoggingActive() {
		return nil
	}

	var dumpBody = false

	if contentTypes, ok := request.Header["Content-Type"]; ok {
		contentType := contentTypes[len(contentTypes)-1]

		dumpBody = shouldDumpBody(contentType)
	}

	dump, err := httputil.DumpRequestOut(request, dumpBody)

	if err != nil {
		return err
	}

	stringDump := string(dump)

	_, err = requestLogFile.WriteString(fmt.Sprintf(`Request-ID: %s
%s
=========================
`, id, stringDump))

	if err != nil {
		return err
	}

	return requestLogFile.Sync()
}

func LogResponse(id string, response *http.Response) error {
	if !IsResponseLoggingActive() {
		return nil
	}

	var dumpBody = false

	if contentTypes, ok := response.Header["Content-Type"]; ok {
		contentType := contentTypes[len(contentTypes)-1]

		dumpBody = shouldDumpBody(contentType)
	}

	dump, err := httputil.DumpResponse(response, dumpBody)

	if err != nil {
		return err
	}

	if id != "" {
		_, err = responseLogFile.WriteString(fmt.Sprintf("Request-ID: %s\n", id))

		if err != nil {
			return err
		}
	}

	stringDump := string(dump)

	_, err = responseLogFile.WriteString(fmt.Sprintf(`%s
=========================
`, stringDump))

	if err != nil {
		return err
	}

	return responseLogFile.Sync()
}

func shouldDumpBody(contentType string) bool {
	if strings.HasPrefix("text/", contentType) {
		return true
	}

	if strings.HasPrefix("application/json", contentType) {
		return true
	}

	if strings.HasPrefix("application/xml", contentType) {
		return true
	}

	return false
}
