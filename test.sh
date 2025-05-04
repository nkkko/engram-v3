#!/bin/bash

# Pretty test script with emoji indicators
echo "🧪 Running all tests..."

# Define packages
core_packages="./internal/lockmanager ./internal/metrics ./internal/notifier ./internal/router ./pkg/client ./pkg/proto"
search_package="./internal/search"
storage_package="./internal/storage"
api_package="./internal/api"

# Run core tests
echo "📦 Running core tests..."
if go test -v $core_packages; then
  echo -e "\n✅ Core tests PASSED\n"
else
  echo -e "\n❌ Core tests FAILED\n"
fi

# Run search tests
echo "🔍 Running search tests..."
if go test -v $search_package; then
  echo -e "\n✅ Search tests PASSED\n"
else
  echo -e "\n❌ Search tests FAILED\n"
fi

# Run storage tests
echo "💾 Running storage tests..."
if go test -v $storage_package; then
  echo -e "\n✅ Storage tests PASSED\n"
else
  echo -e "\n❌ Storage tests FAILED\n"
fi

# Run API tests
echo "🌐 Running API tests..."
if go test -v $api_package; then
  echo -e "\n✅ API tests PASSED\n"
else
  echo -e "\n❌ API tests FAILED\n"
fi

# Run integration tests if they exist
if [ -d "./tests" ]; then
  echo "🧩 Running integration tests..."
  if go test -v ./tests/...; then
    echo -e "\n✅ Integration tests PASSED\n"
  else
    echo -e "\n❌ Integration tests FAILED\n"
  fi
fi

# Summary
echo "🔄 Test Summary"
echo "=============="
passed=0
failed=0

# Count passes and failures
if go test -v $core_packages &>/dev/null; then
  echo "✅ Core tests: PASSED"
  ((passed++))
else
  echo "❌ Core tests: FAILED"
  ((failed++))
fi

if go test -v $search_package &>/dev/null; then
  echo "✅ Search tests: PASSED"
  ((passed++))
else
  echo "❌ Search tests: FAILED"
  ((failed++))
fi

if go test -v $storage_package &>/dev/null; then
  echo "✅ Storage tests: PASSED"
  ((passed++))
else
  echo "❌ Storage tests: FAILED"
  ((failed++))
fi

if go test -v $api_package &>/dev/null; then
  echo "✅ API tests: PASSED"
  ((passed++))
else
  echo "❌ API tests: FAILED"
  ((failed++))
fi

if [ -d "./tests" ]; then
  if go test -v ./tests/... &>/dev/null; then
    echo "✅ Integration tests: PASSED"
    ((passed++))
  else
    echo "❌ Integration tests: FAILED"
    ((failed++))
  fi
fi

echo ""
echo "✅ PASSED: $passed test suites"
echo "❌ FAILED: $failed test suites"

if [ $failed -eq 0 ]; then
  echo -e "\n🎉 ALL TESTS PASSED! 🎉\n"
  exit 0
else
  echo -e "\n❌ SOME TESTS FAILED ❌\n"
  exit 1
fi