TESTSRC = testdata/src
TESTPKG = github.com/gopherd/tools/cmd/gopherlint/${TESTSRC}

.PHONY: all
all: unusedresult final

.PHONY: unusedresult
unusedresult:
	-go run . -unusedresult -unusedresult.types *${TESTPKG}/a.user ./${TESTSRC}/...

.PHONY: final
final:
	-go run . -final ./${TESTSRC}/...
