package entitlements

import (
	"errors"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-cli/bintray/utils"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils/io/httputils"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils/log"
	clientuitls "github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils"
	"fmt"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils/errorutils"
)

func CreateEntitlement(flags *EntitlementFlags, details *utils.VersionDetails) (err error) {
	var path = BuildEntitlementsUrl(flags.BintrayDetails, details)

	if flags.BintrayDetails.User == "" {
		flags.BintrayDetails.User = details.Subject
	}
	data := buildEntitlementJson(flags, true)
	httpClientsDetails := utils.GetBintrayHttpClientDetails(flags.BintrayDetails)
	log.Info("Creating entitlement...")
	resp, body, err := httputils.SendPost(path, []byte(data), httpClientsDetails)
	if err != nil {
		return
	}
	if resp.StatusCode != 201 {
		return errorutils.CheckError(errors.New("Bintray response: " + resp.Status + "\n" + clientuitls.IndentJson(body)))
	}

	log.Debug("Bintray response:", resp.Status)
	log.Info("Created entitlement, details:")
	fmt.Println(clientuitls.IndentJson(body))
	return
}

