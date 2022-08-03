package checks_test

import "testing"

/*
1. Defaults to commit status if checks is not enabled
2. Project Level Policy Check & Checkrun DNE
3. Project Level Policy Check & Checkrun Exists
4. Pending {Plan, Apply}
5. Pending Project Level {Plan, Apply}
6. Approve Policies Command
7. Non Pending Plan Apply, with & without project
8. Error when Checkrun DNE in db
*/

func TestIsValid_DeleteCloneError(t *testing.T) {

}
