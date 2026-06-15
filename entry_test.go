package logrus_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contextKeyType string

func TestEntryWithError(t *testing.T) {
	expErr := fmt.Errorf("kaboom at layer %d", 4711)
	assert.Equal(t, expErr, logrus.WithError(expErr).Data["error"])

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)

	assert.Equal(t, expErr, entry.WithError(expErr).Data["error"])

	tmpKey := logrus.ErrorKey
	logrus.ErrorKey = "err" //nolint:reassign // ignore "reassigning variable ErrorKey in other package logrus (reassign)"
	t.Cleanup(func() {
		logrus.ErrorKey = tmpKey //nolint:reassign // ignore "reassigning variable ErrorKey in other package logrus (reassign)"
	})

	assert.Equal(t, expErr, entry.WithError(expErr).Data["err"])
}

func TestEntryWithContext(t *testing.T) {
	assert := assert.New(t)
	var contextKey contextKeyType = "foo"
	ctx := context.WithValue(context.Background(), contextKey, "bar")

	assert.Equal(ctx, logrus.WithContext(ctx).Context)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)

	assert.Equal(ctx, entry.WithContext(ctx).Context)
}

func TestEntryWithContextCopiesData(t *testing.T) {
	assert := assert.New(t)

	// Initialize a parent Entry object with a key/value set in its Data map
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	parentEntry := logrus.NewEntry(logger).WithField("parentKey", "parentValue")

	// Create two children Entry objects from the parent in different contexts
	var contextKey1 contextKeyType = "foo"
	ctx1 := context.WithValue(context.Background(), contextKey1, "bar")
	childEntry1 := parentEntry.WithContext(ctx1)
	assert.Equal(ctx1, childEntry1.Context)

	var contextKey2 contextKeyType = "bar"
	ctx2 := context.WithValue(context.Background(), contextKey2, "baz")
	childEntry2 := parentEntry.WithContext(ctx2)
	assert.Equal(ctx2, childEntry2.Context)
	assert.NotEqual(ctx1, ctx2)

	// Ensure that data set in the parent Entry are preserved to both children
	assert.Equal("parentValue", childEntry1.Data["parentKey"])
	assert.Equal("parentValue", childEntry2.Data["parentKey"])

	// Modify data stored in the child entry
	childEntry1.Data["childKey"] = "childValue"

	// Verify that data is successfully stored in the child it was set on
	val, exists := childEntry1.Data["childKey"]
	assert.True(exists)
	assert.Equal("childValue", val)

	// Verify that the data change to child 1 has not affected its sibling
	val, exists = childEntry2.Data["childKey"]
	assert.False(exists)
	assert.Empty(val)

	// Verify that the data change to child 1 has not affected its parent
	val, exists = parentEntry.Data["childKey"]
	assert.False(exists)
	assert.Empty(val)
}

func TestEntryWithTimeCopiesData(t *testing.T) {
	assert := assert.New(t)

	// Initialize a parent Entry object with a key/value set in its Data map
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	parentEntry := logrus.NewEntry(logger).WithField("parentKey", "parentValue")

	// Create two children Entry objects from the parent with two different times
	childEntry1 := parentEntry.WithTime(time.Now().AddDate(0, 0, 1))
	childEntry2 := parentEntry.WithTime(time.Now().AddDate(0, 0, 2))

	// Ensure that data set in the parent Entry are preserved to both children
	assert.Equal("parentValue", childEntry1.Data["parentKey"])
	assert.Equal("parentValue", childEntry2.Data["parentKey"])

	// Modify data stored in the child entry
	childEntry1.Data["childKey"] = "childValue"

	// Verify that data is successfully stored in the child it was set on
	val, exists := childEntry1.Data["childKey"]
	assert.True(exists)
	assert.Equal("childValue", val)

	// Verify that the data change to child 1 has not affected its sibling
	val, exists = childEntry2.Data["childKey"]
	assert.False(exists)
	assert.Empty(val)

	// Verify that the data change to child 1 has not affected its parent
	val, exists = parentEntry.Data["childKey"]
	assert.False(exists)
	assert.Empty(val)
}

func TestEntryPanicln(t *testing.T) {
	errBoom := fmt.Errorf("boom time")

	defer func() {
		p := recover()
		assert.NotNil(t, p)

		switch pVal := p.(type) {
		case *logrus.Entry:
			assert.Equal(t, "kaboom", pVal.Message)
			assert.Equal(t, errBoom, pVal.Data["err"])
		default:
			t.Fatalf("want type *Entry, got %T: %#v", pVal, pVal)
		}
	}()

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)
	entry.WithField("err", errBoom).Panicln("kaboom")
}

func TestEntryPanicf(t *testing.T) {
	errBoom := fmt.Errorf("boom again")

	defer func() {
		p := recover()
		assert.NotNil(t, p)

		switch pVal := p.(type) {
		case *logrus.Entry:
			assert.Equal(t, "kaboom true", pVal.Message)
			assert.Equal(t, errBoom, pVal.Data["err"])
		default:
			t.Fatalf("want type *Entry, got %T: %#v", pVal, pVal)
		}
	}()

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)
	entry.WithField("err", errBoom).Panicf("kaboom %v", true)
}

func TestEntryPanic(t *testing.T) {
	errBoom := fmt.Errorf("boom again")

	defer func() {
		p := recover()
		assert.NotNil(t, p)

		switch pVal := p.(type) {
		case *logrus.Entry:
			assert.Equal(t, "kaboom", pVal.Message)
			assert.Equal(t, errBoom, pVal.Data["err"])
		default:
			t.Fatalf("want type *Entry, got %T: %#v", pVal, pVal)
		}
	}()

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)
	entry.WithField("err", errBoom).Panic("kaboom")
}

const (
	badMessage   = "this is going to panic"
	panicMessage = "this is broken"
)

type panickyHook struct{}

func (p *panickyHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.InfoLevel}
}

func (p *panickyHook) Fire(entry *logrus.Entry) error {
	if entry.Message == badMessage {
		panic(panicMessage)
	}

	return nil
}

func TestEntryHooksPanic(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.InfoLevel)
	logger.AddHook(&panickyHook{})

	defer func() {
		p := recover()
		assert.NotNil(t, p)
		assert.Equal(t, panicMessage, p)

		entry := logrus.NewEntry(logger)
		entry.Info("another message")
	}()

	entry := logrus.NewEntry(logger)
	entry.Info(badMessage)
}

func TestEntryWithIncorrectField(t *testing.T) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)

	fn := func() {}
	eWithFunc := entry.WithFields(logrus.Fields{"func": fn})
	eWithFuncPtr := entry.WithFields(logrus.Fields{"funcPtr": &fn})

	assert.Equal(t, `can not add field "func"`, getErr(t, eWithFunc))
	assert.Equal(t, `can not add field "funcPtr"`, getErr(t, eWithFuncPtr))

	eWithFunc = eWithFunc.WithField("not_a_func", "it is a string")
	eWithFuncPtr = eWithFuncPtr.WithField("not_a_func", "it is a string")

	assert.Equal(t, `can not add field "func"`, getErr(t, eWithFunc))
	assert.Equal(t, `can not add field "funcPtr"`, getErr(t, eWithFuncPtr))

	eWithFunc = eWithFunc.WithTime(time.Now())
	eWithFuncPtr = eWithFuncPtr.WithTime(time.Now())

	assert.Equal(t, `can not add field "func"`, getErr(t, eWithFunc))
	assert.Equal(t, `can not add field "funcPtr"`, getErr(t, eWithFuncPtr))
}

func getErr(t *testing.T, e *logrus.Entry) string {
	t.Helper()

	out, err := e.String()
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &m))

	got, _ := m[logrus.FieldKeyLogrusError].(string)
	return got
}

func TestEntryLogfLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buffer)
	logger.SetLevel(logrus.InfoLevel)
	entry := logrus.NewEntry(logger)

	entry.Logf(logrus.DebugLevel, "%s", "debug")
	assert.NotContains(t, buffer.String(), "debug")

	entry.Logf(logrus.WarnLevel, "%s", "warn")
	assert.Contains(t, buffer.String(), "warn")
}

func TestEntryLoggerMutationRace(t *testing.T) {
	tests := []struct {
		doc    string
		mutate func(*logrus.Logger)
	}{
		{doc: "AddHook", mutate: func(l *logrus.Logger) { l.AddHook(noopHook{}) }},
		{doc: "SetBufferPool", mutate: func(l *logrus.Logger) { l.SetBufferPool(nopBufferPool{}) }},
		{doc: "SetFormatter", mutate: func(l *logrus.Logger) { l.SetFormatter(&logrus.TextFormatter{}) }},
		{doc: "SetLevel", mutate: func(l *logrus.Logger) { l.SetLevel(logrus.InfoLevel) }},
		{doc: "SetOutput", mutate: func(l *logrus.Logger) { l.SetOutput(io.Discard) }},
		{doc: "SetReportCaller", mutate: func(l *logrus.Logger) { l.SetReportCaller(true) }},
		{doc: "ReplaceHooks_withHookPresent", mutate: func(l *logrus.Logger) {
			// Replace with a fresh map each time to maximize mutation.
			h := make(logrus.LevelHooks)
			for _, lvl := range logrus.AllLevels {
				h[lvl] = []logrus.Hook{noopHook{}}
			}
			l.ReplaceHooks(h)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			runEntryLoggerRace(t, tc.mutate)
		})
	}
}

type noopHook struct{}

func (noopHook) Levels() []logrus.Level   { return logrus.AllLevels }
func (noopHook) Fire(*logrus.Entry) error { return nil }

type nopBufferPool struct{}

func (nopBufferPool) Get() *bytes.Buffer { return new(bytes.Buffer) }
func (nopBufferPool) Put(*bytes.Buffer)  {}

func runEntryLoggerRace(t *testing.T, mutate func(logger *logrus.Logger)) {
	t.Helper()

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)

	const n = 100

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		for range n {
			_, _ = entry.Bytes()
		}
	}()

	go func() {
		defer wg.Done()
		for range n {
			entry.Info("should not race")
		}
	}()

	go func() {
		defer wg.Done()
		for range n {
			mutate(logger)
		}
	}()

	go func() {
		defer wg.Done()
		for range n {
			entry.Info("should not race")
		}
	}()

	wg.Wait()
}

// reentrantValue is a type whose MarshalJSON method triggers another log call,
// which would deadlock if the logger mutex is held during formatting.
type reentrantValue struct {
	logger *logrus.Logger
}

func (r reentrantValue) MarshalJSON() ([]byte, error) {
	r.logger.Info("reentrant log from MarshalJSON")
	return []byte(`"reentrant"`), nil
}

// TestEntryReentrantLoggingDeadlock verifies that logging from within a field's
// MarshalJSON (or similar serialization callback) does not deadlock.
// This is a regression test for https://github.com/sirupsen/logrus/issues/1448.
func TestEntryReentrantLoggingDeadlock(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		logger.WithFields(logrus.Fields{
			"key": reentrantValue{logger: logger},
		}).Info("outer log message")
	}()

	select {
	case <-done:
		// Success: the log call completed without deadlocking.
		output := buf.String()
		assert.Contains(t, output, "outer log message")
		assert.Contains(t, output, "reentrant log from MarshalJSON")
		assert.Contains(t, output, `"key":"reentrant"`)
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected: reentrant logging from MarshalJSON blocked for 5 seconds")
	}
}

func TestEntryWithModule(t *testing.T) {
	assert := assert.New(t)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)

	moduleEntry := entry.WithModule("auth")
	assert.Equal("auth", moduleEntry.Module)
	assert.Empty(entry.Module)

	emptyModuleEntry := entry.WithModule("")
	assert.Empty(emptyModuleEntry.Module)
}

func TestEntryWithModuleCopiesData(t *testing.T) {
	assert := assert.New(t)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	parentEntry := logrus.NewEntry(logger).WithField("parentKey", "parentValue")

	childEntry1 := parentEntry.WithModule("module1")
	childEntry2 := parentEntry.WithModule("module2")

	assert.Equal("module1", childEntry1.Module)
	assert.Equal("module2", childEntry2.Module)
	assert.Equal("parentValue", childEntry1.Data["parentKey"])
	assert.Equal("parentValue", childEntry2.Data["parentKey"])
	assert.Empty(parentEntry.Module)

	childEntry1.Data["childKey"] = "childValue"
	val, exists := childEntry1.Data["childKey"]
	assert.True(exists)
	assert.Equal("childValue", val)

	_, exists = childEntry2.Data["childKey"]
	assert.False(exists)

	_, exists = parentEntry.Data["childKey"]
	assert.False(exists)
}

func TestEntryWithModulePreservedInChain(t *testing.T) {
	assert := assert.New(t)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger).WithModule("auth")

	entryWithField := entry.WithField("key", "value")
	assert.Equal("auth", entryWithField.Module)

	entryWithError := entry.WithError(fmt.Errorf("error"))
	assert.Equal("auth", entryWithError.Module)

	entryWithContext := entry.WithContext(context.Background())
	assert.Equal("auth", entryWithContext.Module)

	entryWithTime := entry.WithTime(time.Now())
	assert.Equal("auth", entryWithTime.Module)

	entryWithFields := entry.WithFields(logrus.Fields{"a": "b", "c": "d"})
	assert.Equal("auth", entryWithFields.Module)
}

func TestEntryModuleOutputPrefix(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})

	logrus.NewEntry(logger).WithModule("auth").Info("login successful")
	output := buf.String()
	assert.Contains(output, "[auth] level=info msg=\"login successful\"")

	buf.Reset()
	logger.Info("no module log")
	output = buf.String()
	assert.NotContains(output, "[auth]")
	assert.Contains(output, "level=info msg=\"no module log\"")
}

func TestEntryModuleJSONOutputPrefix(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{
		DisableTimestamp: true,
	})

	logrus.NewEntry(logger).WithModule("auth").Info("login successful")
	output := buf.String()

	var fields map[string]any
	err := json.Unmarshal([]byte(output), &fields)
	assert.NoError(err)
	assert.Equal("login successful", fields["msg"])
	assert.Equal("info", fields["level"])
	assert.Equal("auth", fields["module"])
}

func TestEntryModuleSpecialCharacters(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})

	testCases := []struct {
		module   string
		expected string
	}{
		{"my.module", "[my.module] "},
		{"my-module", "[my-module] "},
		{"my_module", "[my_module] "},
		{"module with spaces", "[module with spaces] "},
		{"中文模块", "[中文模块] "},
		{"auth@service", "[auth@service] "},
	}

	for _, tc := range testCases {
		buf.Reset()
		logrus.NewEntry(logger).WithModule(tc.module).Info("test")
		output := buf.String()
		assert.True(strings.HasPrefix(output, tc.expected), "expected prefix %q for module %q, got %q", tc.expected, tc.module, output)
	}
}

func TestEntryModuleEmptyStringClearsModule(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})

	entry := logrus.NewEntry(logger).WithModule("auth")
	buf.Reset()
	entry.Info("has module")
	assert.Contains(buf.String(), "[auth] ")

	entryNoModule := entry.WithModule("")
	buf.Reset()
	entryNoModule.Info("no module")
	assert.NotContains(buf.String(), "[auth] ")
}

func TestEntryModuleLongName(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})

	longModule := strings.Repeat("a", 100)
	logrus.NewEntry(logger).WithModule(longModule).Info("test")
	output := buf.String()
	expectedPrefix := "[" + longModule + "] "
	assert.True(strings.HasPrefix(output, expectedPrefix))
}

func TestEntryModuleConcurrent(t *testing.T) {
	assert := assert.New(t)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	var wg sync.WaitGroup
	n := 100

	for i := range n {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			module := fmt.Sprintf("module-%d", id)
			entry := logrus.NewEntry(logger).WithModule(module)
			for j := range 10 {
				e := entry.WithField("iteration", j)
				assert.Equal(module, e.Module)
				e.Info("concurrent test")
			}
		}(i)
	}

	wg.Wait()
}

func TestEntryModuleDup(t *testing.T) {
	assert := assert.New(t)

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger).WithModule("auth").WithField("key", "value")

	dup := entry.Dup()
	assert.Equal("auth", dup.Module)
	assert.Equal("value", dup.Data["key"])

	dup.Module = "other"
	dup.Data["key"] = "other"
	assert.Equal("auth", entry.Module)
	assert.Equal("value", entry.Data["key"])
}

func TestEntryWithModuleOnGlobalLogger(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	oldOut := logrus.StandardLogger().Out
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})
	t.Cleanup(func() {
		logrus.SetOutput(oldOut)
	})

	logrus.WithModule("global").Info("global module test")
	output := buf.String()
	assert.Contains(output, "[global] ")
}

func TestEntryModuleNotInDataFields(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{
		DisableTimestamp: true,
	})

	logrus.NewEntry(logger).WithModule("auth").Info("test")
	output := buf.String()

	var fields map[string]any
	err := json.Unmarshal([]byte(output), &fields)
	assert.NoError(err)
	_, exists := fields["Module"]
	assert.False(exists, "Module should not be in data fields with capital M")
	assert.Equal("auth", fields["module"], "module should be in json fields with lowercase")
}
