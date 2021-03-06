package utils

import (
	"encoding/json"
	"strings"
	"strconv"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils/log"
	"errors"
	"sort"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils/errorutils"
	"github.com/jfrogdev/jfrog-cli-go/jfrog-client/utils"
)

type SearchParams interface {
	FileGetter
	GetFile() *ArtifactoryCommonParams
}

type SearchParamsImpl struct {
	*ArtifactoryCommonParams
}

func (s *SearchParamsImpl) GetFile() *ArtifactoryCommonParams {
	return s.ArtifactoryCommonParams
}

func SearchBySpecFiles(searchParams SearchParams, flags CommonConf) ([]ResultItem, error) {
	var resultItems []ResultItem
	var itemsFound []ResultItem
	var err error

		switch searchParams.GetSpecType() {
		case WILDCARD, SIMPLE:
			itemsFound, e := AqlSearchDefaultReturnFields(searchParams.GetFile(), flags)
			if e != nil {
				err = e
				return resultItems, err
			}
			resultItems = append(resultItems, itemsFound...)
		case AQL:
			itemsFound, err = AqlSearchBySpec(searchParams.GetFile(), flags)
			if err != nil {
				return resultItems, err
			}
			resultItems = append(resultItems, itemsFound...)
		}
	return resultItems, err
}

func AqlSearchDefaultReturnFields(specFile *ArtifactoryCommonParams, flags CommonConf) ([]ResultItem, error) {
	query, err := createAqlBodyForItem(specFile)
	if err != nil {
		return nil, err
	}
	specFile.Aql = Aql{ItemsFind:query}
	return AqlSearchBySpec(specFile, flags)
}

func AqlSearchBySpec(specFile *ArtifactoryCommonParams, flags CommonConf) ([]ResultItem, error) {
	aqlBody := specFile.Aql.ItemsFind
	query := "items.find(" + aqlBody + ").include(" + strings.Join(GetDefaultQueryReturnFields(), ",") + ")"
	results, err := aqlSearch(query, flags)
	if err != nil {
		return nil, err
	}
	buildIdentifier := specFile.Build
	if buildIdentifier != "" && len(results) > 0 {
		results, err = filterSearchByBuild(buildIdentifier, results, flags)
		if err != nil {
			return nil, err
		}
	}
	return results, err
}

func aqlSearch(aqlQuery string, flags CommonConf) ([]ResultItem, error) {
	json, err := execAqlSearch(aqlQuery, flags)
	if err != nil {
		return nil, err
	}

	resultItems, err := parseAqlSearchResponse(json)
	return resultItems, err
}

func execAqlSearch(aqlQuery string, flags CommonConf) ([]byte, error) {
	client := flags.GetJfrogHttpClient()
	aqlUrl := flags.GetArtifactoryDetails().Url + "api/search/aql"
	log.Debug("Searching Artifactory using AQL query:\n", aqlQuery)

	httpClientsDetails := flags.GetArtifactoryDetails().CreateArtifactoryHttpClientDetails()
	resp, body, err := client.SendPost(aqlUrl, []byte(aqlQuery), httpClientsDetails)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errorutils.CheckError(errors.New("Artifactory response: " + resp.Status + "\n" + utils.IndentJson(body)))
	}

	log.Debug("Artifactory response: ", resp.Status)
	return body, err
}

func GetDefaultQueryReturnFields() []string {
	return []string{"\"name\"", "\"repo\"", "\"path\"", "\"actual_md5\"", "\"actual_sha1\"", "\"size\"", "\"property\"", "\"type\""}
}

func LogSearchResults(numOfArtifacts int) {
	var msgSuffix = "artifacts."
	if numOfArtifacts == 1 {
		msgSuffix = "artifact."
	}
	log.Info("Found", strconv.Itoa(numOfArtifacts), msgSuffix)
}

func parseAqlSearchResponse(resp []byte) ([]ResultItem, error) {
	var result AqlSearchResult
	err := json.Unmarshal(resp, &result)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	return result.Results, nil
}

type AqlSearchResult struct {
	Results []ResultItem
}

type ResultItem struct {
	Repo        string
	Path        string
	Name        string
	Actual_Md5  string
	Actual_Sha1 string
	Size        int64
	Properties  []Property
	Type        string
}

type Property struct {
	Key   string
	Value string
}

func (item ResultItem) GetItemRelativePath() string {
	if item.Path == "." {
		return item.Repo + "/" + item.Name
	}

	url := item.Repo
	url = addSeparator(url, "/", item.Path)
	url = addSeparator(url, "/", item.Name)
	if item.Type == "folder" && !strings.HasSuffix(url, "/") {
		url = url + "/"
	}
	return url
}

func addSeparator(str1, separator, str2 string) string {
	if str2 == "" {
		return str1
	}
	if str1 == "" {
		return str2
	}

	return str1 + separator + str2
}

type AqlSearchResultItemFilter func(map[string]ResultItem, []string) []ResultItem

func FilterBottomChainResults(paths map[string]ResultItem, pathsKeys []string) []ResultItem {
	var result []ResultItem
	sort.Sort(sort.Reverse(sort.StringSlice(pathsKeys)))
	for i, k := range pathsKeys {
		if i == 0 || !IsSubPath(pathsKeys, i, "/") {
			result = append(result, paths[k])
		}
	}

	return result
}

func FilterTopChainResults(paths map[string]ResultItem, pathsKeys []string) []ResultItem {
	sort.Strings(pathsKeys)
	for _, k := range pathsKeys {
		for _, k2 := range pathsKeys {
			prefix := k2
			if paths[k2].Type == "folder" &&  !strings.HasSuffix(k2, "/") {
				prefix += "/"
			}

			if k != k2 && strings.HasPrefix(k, prefix) {
				delete(paths, k)
				continue
			}
		}
	}

	var result []ResultItem
	for _, v := range paths {
		result = append(result, v)
	}

	return result
}

// Reduce Dir results by using the resultsFilter
func ReduceDirResult(searchResults []ResultItem, resultsFilter AqlSearchResultItemFilter) []ResultItem {
	paths := make(map[string]ResultItem)
	pathsKeys := make([]string, 0, len(searchResults))
	for _, file := range searchResults {
		if file.Name == "." {
			continue
		}

		url := file.GetItemRelativePath()
		paths[url] = file
		pathsKeys = append(pathsKeys, url)
	}
	return  resultsFilter(paths, pathsKeys)
}