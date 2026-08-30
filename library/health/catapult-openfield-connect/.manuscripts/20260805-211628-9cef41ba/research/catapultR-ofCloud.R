##----------------------------------------------------
# ofCloud.R - a wrapper for OpenField cloud Connect APIs in R.
# The API is documented at https://docs.connect.catapultsports.com/ .
##----------------------------------------------------

#library(httr)     # GET(), POST(), add_headers()
#library(crul)     # Async() 
#library(jsonlite) # fromJSON(), toJSON()
#library(dplyr)
#library(stringr)
#library(purrr)    # safely()

# test stage
regionToBaseURL_map_alpha <- c("APAC"="au-alpha.catapultsports.com", "EMEA"="eu-alpha.catapultsports.com", "America"="us-alpha.catapultsports.com", "China"="connect-cn-alpha.catapultsports-cn.com")
regionToBaseURL_map_beta <- c("APAC"="au-beta.catapultsports.com", "EMEA"="eu-beta.catapultsports.com", "America"="us-beta.catapultsports.com", "China"="connect-cn-beta.catapultsports-cn.com")

# production
regionToBaseURL_map_main <- c("APAC"="openfield.catapultsports.com", "EMEA"="eu.catapultsports.com", "America"="us.catapultsports.com", "China"="connect-cn.catapultsports-cn.com")

listRegionToBaseURL_map <- list("alpha" = regionToBaseURL_map_alpha, "beta" = regionToBaseURL_map_beta, "main" = regionToBaseURL_map_main)

escapeForJSON<-function(s)
{
  # escape backslash with double backslash (remember, the second parameter in str_replace_all() is the regular expression)
  s <- stringr::str_replace_all(s, "\\\\", "\\\\\\\\")
  # OFC-4716: escape double quote symbol " with \" (remember, the second parameter in str_replace_all() is the regular expression)
  s <- stringr::str_replace_all(s, '"', '\\\\"')
}

# validate response from crul::Async()$get()[j]
validate_crul_http_response <- function(httpResponse)
{
  httpResponse$raise_for_status()   # stop if status >= 300
  httpResponse$raise_for_ct_json()  # stop if response content-type is not application/json
  stopifnot(httpResponse$success() == TRUE)
  return(TRUE)
}
safe_validate_crul_http_response <- purrr::safely(validate_crul_http_response)

# deliberately do not export this class with #' @export. 
# TODO: cf. httr::Token-class created with oauth2.0_token(); there is no integration with for OAuth2.0 flow yet; 
# see browseVignettes("httr"), 'Managing secrets' on caching secrets (this is TODO for catapultR - cache the token in path.expand('~')).
ofCredentials <- R6::R6Class("ofCredentials", lock_class = TRUE,
  private = list
  (
    name = NULL,
    password = NULL,
    region = NULL,
    stage = NULL,
    explicitURL = NULL,
    secure = TRUE,
    clientID = NULL,
    clientSecret = NULL,
    token = NULL,
    refresh_token = NULL,
    tokenTime = NULL,
    tokenExpireTime = NULL,
    traceURL = NULL,
    apiStatus = NULL,      # last API call status
    apiMessage = NULL,     # last API call error message
    apiTimeout = NULL,     # seconds
    modules = NULL         # as returned by ofCloudGetModules()
  ),
  public = list(
      initialize = function(name, password, region, stage="main", clientID, clientSecret, traceURL = FALSE, 
                            apiStatus = 0, apiMessage = "", apiTimeout = 60, modules = NULL, explicitURL = NULL, secure = TRUE) 
    {
      stopifnot(is.character(name), length(name) == 1)
      stopifnot(is.character(password), length(password) == 1)
      if (is.null(explicitURL)) {
          stopifnot(is.character(region), length(region) == 1)
          stopifnot(region %in% names(regionToBaseURL_map_main))
          stopifnot(is.character(stage), length(stage) == 1)
          stopifnot(stage %in% names(listRegionToBaseURL_map))
      }
      private$explicitURL <- explicitURL
      private$secure <- secure
      private$name <- name
      private$password <- password
      private$region <- region
      private$stage <- stage
      private$clientID <- clientID
      private$clientSecret <- clientSecret
      private$traceURL <- traceURL
      private$apiStatus <- apiStatus
      private$apiMessage <- apiMessage
      private$apiTimeout <- apiTimeout
      private$modules <- modules
      private$tokenTime <- 0
      private$tokenExpireTime <- 0
    },
    resetApiStatus = function(){private$apiStatus = 0; private$apiMessage = ""},
    safeLogin = function(refresh = FALSE)
    {
      curTime <- as.integer(Sys.time())
	  if (refresh)
	  {
		  lst <- list(client_id = private$clientID,
					  client_secret = private$clientSecret,
					  refresh_token = private$refresh_token)
		  strURL <- self$getURL("/api/v6/oauth/refresh")
	  }
	  else
	  {
		  lst <- list(username = private$name,  
					  password = private$password,
					  grant_type = "password", 
					  client_id = private$clientID,
					  client_secret = private$clientSecret)
		  strURL <- self$getURL("/api/v6/oauth/token")
	  }
      safe_POST <- purrr::safely(httr::POST)

      # OF cloud login with POST
      sBody <- jsonlite::toJSON(lst, auto_unbox = TRUE)
                            
      if (private$traceURL)
        print(strURL)
      self$resetApiStatus()
      response <- safe_POST(url = strURL,
                            body = sBody,   # jsonlite::toJSON(lst) is not helpful here
                            httr::add_headers(.headers = c('cache-control' = 'no-cache',
                                                           'Content-Type' = 'application/json',
                                                           'Accept' = 'application/json')),
                            httr::timeout(private$apiTimeout),
                            handle=httr::handle('')   # OFC-707 - suppress cookies by using a new handle
                           )
      if (!is.null(response$error))
      {
        return(response)
      }
      private$apiStatus <- httr::status_code(response$result)
      if (private$apiStatus != 200)
      {
        private$apiMessage <- httr::content(response$result, "text", encoding = "UTF-8")
        return(list(error = stringr::str_c("status_code = ", private$apiStatus, ". ", private$apiMessage), 
                    result = self))
      }
      safe_content <- purrr::safely(httr::content)
      response <- safe_content(response$result)
      if (!is.null(response$error))
        return(response)
      response <- response$result
      if (class(response) != "list")
      {
        return(list(error = response, result = NULL))
      }
      if (length(which(names(response)=="access_token")) == 0) 
      {
        return(list(error = "access_token is missing", result = NULL))
      }
      if (length(which(names(response)=="expires_in")) == 0) 
      {
        return(list(error = "expires_in is missing", result = NULL))
      }
      if (length(which(names(response)=="refresh_token")) == 0) 
      {
        return(list(error = "refresh_token is missing", result = NULL))
      }
      private$token <- response$access_token
      private$tokenTime <- curTime
      private$tokenExpireTime <- curTime + response$expires_in
	  private$refresh_token <- response$refresh_token
	  private$password <- NULL  		# refresh_token should be used to refresh the token
      return(list(error = NULL, result = self))
    }, 
    safe_ofCloudValidateToken = function(minTimeToExpiry = 3)
    {
      # Is the token still valid?
      curTime <- as.integer(Sys.time())
      if (curTime + minTimeToExpiry < private$tokenExpireTime) 
	  {
        self$resetApiStatus()
        return(list(error = NULL, result = self))
	  }
	  # refresh the token if the current token has expired or is about to expire
	  if (is.null(private$refresh_token))
		return(list(error = "Cannot refresh token, refresh_token is not set", result = NULL))

	  return(self$safeLogin(refresh = TRUE))
    },
	getAuthorizationBearer = function() # Bearer in HTTP Authorization header
	{
		if (!private$secure)
		{
			return("")
		}
		return(stringr::str_c("Bearer ", stringr::str_replace_all(self$getToken(), "\"", "")))
	},
    httrGet = function(sURL, contentType = "json")
    {
      self$resetApiStatus()
      sURL <- self$getURL(sURL)
      if (self$getTrace())		# TODO: consider httr::with_verbose()
        print(sURL)
      # Consider adding user_agent(stringr::str_c("catapultR ", as.character(packageVersion("catapultR")))) 
      # in httr::GET/POST parameters to change "HTTPUserAgent" request header in the request scope;
      # similar with crul::Async$new by passing headers = list(HTTPUserAgent = "...").
      # Note however, a client might have added options(HTTPUserAgent = ..) to change the header in the session scope. 
      r <- httr::GET(sURL, 
                     httr::add_headers("Authorization" = self$getAuthorizationBearer()),
                     httr::timeout(private$apiTimeout),
                     handle=httr::handle(''))  # OFC-707 - suppress cookies by using a new handle
      private$apiStatus <- r$status_code   # 200 - OK, 4xx - client error (404 - file not found), 500 - server error
      if (httr::http_error(r))
      {
        private$apiMessage <- as.character(r)
        if (r$status_code == 429)
          private$apiMessage <- "Too Many Attempts"      # throttle request
        stop(stringr::str_c("status_code = ", r$status_code, ". ", private$apiMessage), call. = FALSE)
      }
      ret <- switch(contentType, 
                    json = {
                              s <- suppressMessages(jsonlite::fromJSON(httr::content(r, as="text"))) # suppress 'No encoding supplied:
                              if (length(s) == 0)                                                    # defaulting to UTF-8'
                                s = NULL
                              s
                           },
                    text = httr::content(r, as = "text"),
                    raw = httr::content(r, type = "raw"),
                    httr::content(r, as=contentType)
                    )
      return(ret)
    },
    httrPost = function(sURL, sBody, patch = FALSE, contentType = "json")
    {
      sURL <- self$getURL(sURL)
      if (self$getTrace())	# TODO: consider httr::with_verbose()
      {
        print(sURL)
        print(sBody)
      }
      self$resetApiStatus()
	  f <- ifelse(patch, httr::PATCH, httr::POST)
      r <- f( url = sURL,
			  body = sBody,
			  httr::add_headers(.headers = c('Authorization'= self$getAuthorizationBearer(),
			 'cache-control' = 'no-cache',
			 'Content-Type' = 'application/json',
			 'Accept' = 'application/json')),
			  httr::timeout(private$apiTimeout),
			  handle=httr::handle('')             # OFC-707 - suppress cookies by using a new handle
			  )
      private$apiStatus <- r$status_code   # 200 - OK, 4xx - client error (404 - file not found), 500 - server error
      if (httr::http_error(r))
      {
        private$apiMessage <- as.character(r)
        if (r$status_code == 429)
          private$apiMessage <- "Too Many Attempts"      # throttle request
        stop(stringr::str_c("status_code = ", r$status_code, ". ", private$apiMessage), call. = FALSE)
      }
      ret <- switch(contentType, 
                    json = suppressMessages(jsonlite::fromJSON(httr::content(r, as="text"))), # suppress 'No encoding supplied: defaulting to UTF-8'
                    text = httr::content(r, as = "text"),
                    raw = httr::content(r, type = "raw"),
                    httr::content(r, as=contentType)
                    )
	  if (length(ret) == 0)
	  {
		ret <- NULL
	  }
      return(ret)
    },
    print = function(...) {
      cat("ofCredentials: \n")
      cat("  Name: ", private$name, "\n", sep = "")
	  if (is.null(private$explicitURL)) {
		  cat("  Region:  ", private$region, "\n", sep = "")
		  cat("  Stage:  ", private$stage, "\n", sep = "")
	  } else {
		  cat("  explicitURL:  ", private$explicitURL, "\n", sep = "")
		  cat("  secure:  ", private$secure, "\n", sep = "")
	  }
      cat("  TokenTime:  ", private$tokenTime, "\n", sep = "")
      cat("  TokenExpireTime:  ", private$tokenExpireTime, "\n", sep = "")
      cat("  ApiStatus:  ", private$apiStatus, "\n", sep = "")
      cat("  ApiMessage:  ", private$apiMessage, "\n", sep = "")
      cat("  ApiTimeout:  ", private$apiTimeout, "\n", sep = "")
      invisible(self)
    },
    getToken = function(){
      return(private$token)
    },
    getRefreshToken = function(){
      return(private$refresh_token)
    },    
	getTokenExpireTime = function(){
      return(private$tokenExpireTime)
    },
    getName = function(){
      return(private$name)
    },
    getRegion = function(){
      return(private$region)
    },
    getStage = function(){
      return(private$stage)
    },
    setToken = function(token, region = NULL, stage = NULL, tokenExpireTime = 0, refresh_token = NULL){   # advanced feature
      private$token <- token
	  private$tokenTime <- NULL
      if (!is.null(refresh_token)) private$refresh_token <- refresh_token
      if (!is.null(region)) private$region <- region
      if (!is.null(stage)) private$stage <- stage
      private$tokenExpireTime <- tokenExpireTime
      private$password <- NULL
      invisible(self)
    },
    regionToURL = function(){
	  if (is.null(private$explicitURL)) {
		return(listRegionToBaseURL_map[[private$stage]][private$region])
	  } else {
		return(private$explicitURL)
	  }
    },
	getURL = function(sURL){
		base <- ifelse(private$secure, "https://", "http://")
		stringr::str_c(base, self$regionToURL(), sURL)
	},
    getTrace = function(){
      return(private$traceURL)
    },
    setTrace = function(b){
      private$traceURL <- b
      invisible(self)
    },
    getApiStatus = function(){
      return(private$apiStatus)
    },
    getApiMessage = function(){
      return(private$apiMessage)
    },
    getApiTimeout = function(){
      return(private$apiTimeout)
    },
    setApiTimeout = function(x){
      private$apiTimeout <- x
      invisible(self)
    },
    getModules = function(){
      return(private$modules)
    },
    validateModules = function(m, enforce = FALSE){
      if (is.null(private$modules) || enforce)
      {
        private$modules <- ofCloudGetModulesImp(self)
      }
      return(all(m %in% private$modules))
    }
  )
)

#' OF cloud login
#'
#' Implements OF cloud login with basic authentication, without appending user name and password to URL.
#'
#' @export
#' @param sRegion a region from the set \code{"APAC", "EMEA", "America", "China"}
#' @param sName a User Name
#' @param sPwd a User Password
#' @param sStage a production (\code{"main"}, default), or a test stage (\code{"alpha", "beta"})
#' @param sClientID,sClientSecret should keep default values unless specifically instructed by Catapult Sports
#' @param traceURL controls API tracing
#' @return \code{ofCloudGetToken} returns ofCredentials R6 object. 
#' @examples
#' token <- ofCloudGetToken("APAC", "sergey", "password", "clientID", "clientSecret")
ofCloudGetToken<-function(sRegion, sName, sPwd, sStage = "main", sClientID, sClientSecret, traceURL = FALSE)
{
  result <- safe_ofCloudGetToken(sRegion, sName, sPwd, sStage, sClientID, sClientSecret, traceURL)
  if (!is.null(result$error))
    stop(result$error)
  return(result$result)
}

#' @describeIn ofCloudGetToken a safe version of the function that returns a list with 'error' and 'result'
#' @export
#' @return \code{safe_ofCloudGetToken} returns a list with \code{result} with ofCredentials R6 object if successful 
#' or \code{error} if failed. 
safe_ofCloudGetToken<-function(sRegion, sName, sPwd, sStage = "main", sClientID, sClientSecret, traceURL = FALSE)
{
  credentials <- ofCredentials$new(name = sName, password = sPwd, region = sRegion, stage = sStage, clientID = sClientID,
                                   clientSecret = sClientSecret, traceURL = traceURL)
  return(credentials$safeLogin())
}

#' @describeIn ofCloudGetToken OF cloud login when the region is not known, by trying all possible regions
#' @export
#' @return \code{safe_ofCloudGetTokenEx} returns a list with \code{result} with ofCredentials R6 object if successful 
#' or \code{error} if failed.  
safe_ofCloudGetTokenEx<-function(sName, sPwd, sStage = "main", sClientID, sClientSecret, traceURL = FALSE)
{
  for (sRegion in names(regionToBaseURL_map_main))
  {
    result <- safe_ofCloudGetToken(sRegion, sName, sPwd, sStage, sClientID, sClientSecret, traceURL)
    if (is.null(result$error))
    {
      return(result)
    }
  }
  return(result)
}

#' @describeIn ofCloudGetToken refresh token if necessary
#'
#' Validates a token in case it has expired.
#' 
#' @export
#' @return \code{safe_ofCloudValidateToken} returns a list with \code{result} with ofCredentials R6 object if successful 
#' or \code{error} if failed.  
#' @param credentials R6 object as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param minTimeToExpiry minimum time required before the token expires (seconds)
#' @examples
#' \dontrun{
#'  safe_ofCloudValidateToken(credentials)
#' }
safe_ofCloudValidateToken <- function(credentials, minTimeToExpiry = 3)
{
  stopifnot(minTimeToExpiry >= 0)
  return(credentials$safe_ofCloudValidateToken(minTimeToExpiry))
}

#' @describeIn ofCloudGetToken create ofCredentials object from a given token string
#'
#' The token object created with \code{ofCloudCreateToken} cannot be renewed with \code{safe_ofCloudValidateToken} unless \code{refresh_token} is provided.
#' 
#' @export
#' @param sToken a token string. Login into openfield.catapultsports.com and navigate to Settings -> API Tokens to create a token (\code{apiScope:catapultR} module is required for the account). Select \code{Summary Data}, \code{Sensor Data} and \code{CatapultR} scopes.
#' @param tokenExpireTime token expire time as POSIX time in seconds since the start of the epoch, designed to match the token Expiration Date from the OF Cloud -> API Tokens screen
#' @param refresh_token a refresh token string
#' @return \code{ofCloudCreateToken} returns ofCredentials R6 object.  
ofCloudCreateToken<-function(sToken, sRegion, sStage = "main", tokenExpireTime = 0, traceURL = FALSE, refresh_token = NULL)
{
  credentials <- ofCredentials$new(name = "", password = "", region = sRegion, stage = sStage, 
                                   clientID = "", clientSecret = "", traceURL = traceURL)
  credentials$setToken(sToken, tokenExpireTime = tokenExpireTime, refresh_token = refresh_token)
  return(credentials)
}

#' @describeIn ofCloudGetToken create ofCredentials object from a given token string for an explicitly specified URL
#'
#' The token object created with \code{ofCloudCreateToken} cannot be renewed with \code{safe_ofCloudValidateToken} unless \code{refresh_token} is provided.
#' 
#' @export
#' @param sToken a token string. Login into openfield.catapultsports.com and navigate to Settings -> API Tokens to create a token (\code{apiScope:catapultR} module is required for the account). Select \code{Summary Data}, \code{Sensor Data} and \code{CatapultR} scopes.
#' @param explicitURL an explicitly specified URL for the APIs, e.g. 'openfield.catapultsports.com', facilitates usage inside AWS Cloud
#' @param tokenExpireTime token expire time as POSIX time in seconds since the start of the epoch, designed to match the token Expiration Date from the OF Cloud -> API Tokens screen
#' @param refresh_token a refresh token string
#' @param secure if \code{TRUE} then use \code{https} and pass \code{Bearer} into \code{Authorization} HTTP header. Use \code{secure = FALSE} in the context of private APIs that run behind VPN or in production; in this case \code{sToken} is not used.
#' @return \code{ofCloudCreateToken} returns ofCredentials R6 object.  
ofCloudCreateTokenWithURL<-function(sToken, explicitURL, tokenExpireTime = 0, traceURL = FALSE, refresh_token = NULL, secure = TRUE)
{
  credentials <- ofCredentials$new(name = "", password = "", region = NULL, stage = NULL, 
                                   clientID = "", clientSecret = "", traceURL = traceURL, explicitURL = explicitURL, secure = secure)
  credentials$setToken(sToken, tokenExpireTime = tokenExpireTime, refresh_token = refresh_token)
  return(credentials)
}

#' get all athletes associated with the OF account
#'
#' \code{ofCloudGetAthletes} returns all athletes associated with the OF account
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetAthletes} returns all athletes associated with the OF account, or \code{NULL} if there are no athletes. 
#' @examples
#' \dontrun{
#' df <- ofCloudGetAthletes(credentials)
#' }
#' @family athlete details access APIs
ofCloudGetAthletes<-function(credentials)
{
  s <- credentials$httrGet("/api/v6/athletes")  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' get all parameters associated with the OF account
#'
#' Returns all parameters associated with the OF account.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetParameters} returns all parameters associated with the OF account, or \code{NULL} if there are no parameters (this should not happen). 
#' @examples
#' \dontrun{
#' df <- ofCloudGetParameters(credentials)
#' }
#' @family cloud metadata access APIs
ofCloudGetParameters<-function(credentials)
{
  s <- credentials$httrGet("/api/v6/parameters")  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

extractVenueAndSyncStatus <- function(df, syncStatus = FALSE)
{
  venue <- df$venue; df$venue <- NULL
  colnames(venue) <- stringr::str_c("venue_", colnames(venue))
  df <- as.data.frame(cbind(df, venue), stringsAsFactors = FALSE)
  
  if (syncStatus)
  {
	synced <- purrr::map_dbl(seq_len(NROW(df)), function(j)
													{
														x <- df$actdic_status[[j]]$sd$munged$synced
														ifelse(is.null(x), NA, x) 
													})  
	unsynced <- purrr::map_dbl(seq_len(NROW(df)), function(j)
													{
														x <- df$actdic_status[[j]]$sd$munged$unsynced
														ifelse(is.null(x), NA, x) 
													})  
	df$actdic_status <- NULL
	df <- as.data.frame(cbind(df, synced = synced, unsynced = unsynced), stringsAsFactors = FALSE)
  }
  
  return(df)
}

#' get activities
#'
#' \code{ofCloudGetActivities} returns a data frame with all activities associated with the OF account, optionally filtered by a time range (\code{from, to}).
#'
#' The returned data frame includes the following attributes:\cr
#'   \code{id, game_id, name, start_time, end_time, periods, tags, tag_list}.
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param from,to POSIX time in seconds since the start of the epoch. \code{NA} by default.
#'        Can be specified either as \code{(from, to)} or as a half-open interval \code{(from,)} or \code{(,to)}.
#' @param syncStatus if \code{TRUE} then the returned data frame has two additional columns, \code{synced} and \code{unsynced}, with the number of SD files in activity that are respectively synchronised / not synchronised. 
#' @return \code{ofCloudGetActivities} returns a data frame with activities for a given time interval, or \code{NULL} if there are no activities. 
#' @examples
#' \dontrun{
#' to <- as.integer(Sys.time()); from <- to - 7*24*60*60
#' activities <- ofCloudGetActivities(credentials, from, to)
#' }
ofCloudGetActivities<-function(credentials, from=NA, to=NA, syncStatus = FALSE)
{
  # uncomment below to proceed with crul 
  #res <- ofCloudGetActivitiesEx(credentials, from, to)[[1]]
  #if (!is.null(res$error))
  #  stop(res$error)  # stop if error is present
  #return(res)
  # proceed with httr
  sURL <- "/api/v6/activities"
  sep <- "?"
  if (!is.na(from) && !is.na(to))
  {
    stopifnot(from < to)
    sURL <- stringr::str_c(sURL, "?startTime=", from, "&endTime=", to)
    sep <- "&"
  }
  else
  {
    if (!is.na(from))
    {
      sURL <- stringr::str_c(sURL, "?startTime=", from)
	  sep <- "&"
    }
    if (!is.na(to))
    {
      sURL <- stringr::str_c(sURL, "?endTime=", to)
      sep <- "&"
    }
  }
  if (syncStatus)
  {
      sURL <- stringr::str_c(sURL, sep, "include=actdic_status")
  }
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  if (is.null(s))   # no activities
    return(NULL)
  
  s <- extractVenueAndSyncStatus(s, syncStatus)
  return(s)
}

#' @describeIn ofCloudGetActivities get activities. The function treats \code{from}, \code{to} as vectors and triggers one cloud API call 
#' for each \code{(from[j], to[j])} interval in parallel. 
#'
#' Be mindful not to overload the server with excessive number of requests. Keep the number of intervals passed into
#' \code{ofCloudGetActivitiesEx} reasonable. Otherwise, the requests might be throttled on the server.
#'
#' @export
#' @return \code{ofCloudGetActivitiesEx} returns a list with an entry for each \code{(from[j], to[j])} interval, 
#' with the entry content matching ofCloudGetActivities() return value.
#' The function does not throw if a HTTP call fails for any interval. \code{error} will be present in the returned list in an entry 
#' for an interval with a failed call. 
ofCloudGetActivitiesEx<-function(credentials, from, to, syncStatus = FALSE)
{
  # validate from, to
  stopifnot(length(from) == length(to))
  for (j in seq_along(from))
  {
    if (!is.na(from[j]) && !is.na(to[j]))
      stopifnot(from[j] < to[j])
  }
  
  sURLs <- sapply(seq_along(from), function(j)
  {
    sURL <- credentials$getURL("/api/v6/activities")
    sep <- "?"
    if (!is.na(from[j]) && !is.na(to[j]))
    {
      sURL <- stringr::str_c(sURL, "?startTime=", from[j], "&endTime=", to[j])
      sep <- "&"
    }  
    else
    {
      if (!is.na(from[j]))
      {
        sURL <- stringr::str_c(sURL, "?startTime=", from[j])
        sep <- "&"
      }
      if (!is.na(to[j]))
      {
        sURL <- stringr::str_c(sURL, "?endTime=", to[j])
        sep <- "&"
      }
    }
	if (syncStatus)
	{
	  sURL <- stringr::str_c(sURL, sep, "include=actdic_status")
	}
    if (credentials$getTrace())
    {  
      print(sURL)
    }
    return(sURL)
  })

  credentials$resetApiStatus()
  crul::set_opts(timeout_ms = credentials$getApiTimeout()*1000)
  cc <- crul::Async$new(urls = sURLs,
                        headers = list("Authorization" = credentials$getAuthorizationBearer()),
                        )
  # Rarely, if OpenField cloud capacity is exhausted, cc$get() triggers 'Resolving timed out after 10000 milliseconds' error. 
  res <- cc$get() 

  x <- lapply(res, function(z){
                                res <- safe_validate_crul_http_response(z)
                                if (!is.null(res$error))
                                  return(list(error = paste(res$error, suppressMessages(z$parse()))))
                                if (z$success() != TRUE)    # (status code > 201) && (status code < 300)
                                {
                                  warning(paste(z$status_http()$status_code, z$status_http()$message, z$status_http()$explanation))
                                }
                                s <- jsonlite::fromJSON(z$parse("UTF-8"))# TODO:s <- jsonlite::fromJSON(z$parse("UTF-8"),flatten=TRUE)
                                if (length(s) == 0)  # no activities
                                  return(NULL)
                                return(extractVenueAndSyncStatus(s, syncStatus))  
                              })
  return(x)
}

#' @describeIn ofCloudGetActivities get periods for an activity
#' @export
#' @return \code{ofCloudGetPeriods} returns a data frame with period details for a given activity, or \code{NULL} if there are no periods. 
ofCloudGetPeriods<-function(credentials, activity_id)
{
  sURL <- stringr::str_c("/api/v6/activities/", activity_id, "/periods")
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' @describeIn ofCloudGetActivities get details for an activity
#' @export
#' @return \code{ofCloudGetActivity} returns a list with details for a given activity. 
ofCloudGetActivity<-function(credentials, activity_id, syncStatus = FALSE)
{
  sURL <- stringr::str_c("/api/v6/activities/", activity_id, "?embed=all")
  if (syncStatus)
  {
    sURL <- stringr::str_c(sURL, "&include=actdic_status")
  }
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  #return(extractVenueAndSyncStatus(s, syncStatus))  # as.data.frame() might fail with 'arguments imply differing number of rows'
  return(s)                 # return the list
}

# legacy route
ofCloudGetAthleteDevicesInActivityOld<-function(credentials, activity_id)
{
  # TODO: data_source is hardcoded as 'munged'. Test adding 'data_source=live' to the URL (as an API parameter) for live period/activity (having module apiScope:live verified),
  # similar to activities/{activityId}/athletes/{athleteId}/sensor below.
  # Note, ‘Summary Data’ API scope alone (without 'sensor-read-only') allows for ofCloudGetAthletesInActivity() [api/v6/activities/<id>/athlete], but does not allow for ofCloudGetAthleteDevicesInActivity() [/api/v6/sensor/devices?activity_id=<id>];
  # the latter requires 'Sensor Data' API scope.  
  sURL <- stringr::str_c("/api/v6/sensor/devices?activity_id=", activity_id)
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' get device id, athlete id and other details for a given activity or period
#'
#' \code{ofCloudGetAthleteDevicesInActivity} returns a data frame with \code{device_id}, \code{athlete_id} and \code{activity_dictionaries} for a given activity.\cr
#' \code{\link{ofCloudGetAthletesInActivity}} and \code{\link{ofCloudGetAthletesInPeriod}} return \code{athlete_id} but do not return \code{device_id}.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param activity_id as returned by \code{\link{ofCloudGetActivities}}
#' @return \code{ofCloudGetAthleteDevicesInActivity} returns a data frame with \code{device_id}, \code{athlete_id} and \code{activity_dictionaries} for a given activity, or \code{NULL} if there are no rows to return. 
#' @examples
#' \dontrun{
#' to <- as.integer(Sys.time()); from <- to - 7*24*60*60
#' activities <- ofCloudGetActivities(credentials, from, to)
#' j <- 1   # the activity of interest
#' dfAD <- ofCloudGetAthleteDevicesInActivity(credentials, activities$id[j])
#' dfActDict <- bind_rows(dfAD$activity_dictionaries)
#' }
#' @family athlete details access APIs
ofCloudGetAthleteDevicesInActivity<-function(credentials, activity_id)
{
  sURL <- stringr::str_c("/api/v6/activities/", activity_id, "/devices")
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' @describeIn ofCloudGetAthleteDevicesInActivity returns a data frame with device id, athlete and team details for a given period.\cr
#' @param period_id as returned by \code{\link{ofCloudGetPeriods}} or \code{\link{ofCloudGetActivities}}
#' @export
ofCloudGetAthleteDevicesInPeriod<-function(credentials, period_id)
{
  # TODO: data_source is hardcoded as 'munged'. Test adding 'data_source=live' to the URL (as an API parameter) for live period/activity (having module apiScope:live verified),
  # similar to activities/{activityId}/athletes/{athleteId}/sensor below
  sURL <- stringr::str_c("/api/v6/sensor/devices?period_id=", period_id)
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

ofCloudGetActivityPeriodSensorData<-function(credentials, athlete_id, id, bActivity, from, to, page_ordinal, page_size,
                                             stream_type, parameters, field, useCrul)
{
  stopifnot(!is.na(id))
  stopifnot(length(athlete_id) == length(unique(athlete_id)))
  stopifnot(all(names(field) %in% c("centre_latitude", "centre_longitude", "rotation", "length", "width")))
  stopifnot((length(athlete_id) == 1) || useCrul) # if !useCrul then must satisfy (length(athlete_id) == 1)
  
  sURLs <- sapply(seq_along(athlete_id), function(j)
  {
	# TODO: data_source is hardcoded as 'munged'. Test adding 'data_source=live' to the URL (as an API parameter) for live period/activity (having module apiScope:live verified).
	# See https://catapultgroup.slack.com/archives/C6A1Z0WQH/p1637537014380700?thread_ts=1637535599.378600&cid=C6A1Z0WQH .
	# Similar to sensor/devices?activity_id= and /api/v6/sensor/devices?period_id= above
    sURL <- stringr::str_c("/api/v6/", 
                           ifelse(bActivity, "activities/", "periods/"), 
                           id, "/athletes/", athlete_id[j], "/sensor")
    sep <- "?"
    if (!is.na(from) && !is.na(to))
    {
      stopifnot(from < to)
    }
    if (!is.na(from) && (as.integer(from) != 0))
    {
      sURL <- stringr::str_c(sURL, sep, "start_time=", from)
      sep <- "&"
    }
    if (!is.na(to))
    {
      sURL <- stringr::str_c(sURL, sep, "end_time=", to)
      sep <- "&"
    }
    if (!is.na(stream_type))
    {
      stopifnot(stream_type %in% c("gps", "lps"))
      sURL <- stringr::str_c(sURL, sep, "stream_type=", stream_type)
      sep <- "&"
    }
    if (!all(is.na(parameters)))
    {
      stopifnot(all(parameters %in% c("ts", "cs", "lat", "long", "o", "v", "a", "hr", "pl", "mp", "sl", "xy", "alt", "pq", "ref", "hdop", "rv", "face", "sc")))
      sURL <- stringr::str_c(sURL, sep, "parameters=", paste0(parameters, collapse=","))
      sep <- "&"
    }
    if (!is.na(page_ordinal) || !is.na(page_size))
    {
      stopifnot(!is.na(page_ordinal) && !is.na(page_size))
      sURL <- stringr::str_c(sURL, sep, "page=", page_ordinal, "&page_size_seconds=", page_size)
      sep <- "&"
    }
    sURL <- stringr::str_c(sURL, sep, "nulls=1"); sep <- "&"  # OW-1290: return NA for invalid (no lock) positional data
    
    purrr::walk(seq_along(field), function(j)
                                  {
                                    fieldName <- stringr::str_c("field_", names(field)[j])
                                    sURL <<- stringr::str_c(sURL, sep, fieldName, "=", field[[j]]); sep <- "&"
                                  })
    return(sURL)
  })

  if (!useCrul) # proceed with httr
  {
    stopifnot(length(sURLs) == 1)
    x <- credentials$httrGet(sURLs)  # httrGet() calls stop() if (credentials$apiStatus != 200)
    if (is.null(x))
    {
      return(NULL)
    }
    if (length(x$data) == 0)   # OW-1054: API might return an empty list
      return(NULL)
    if (length(x$data[[1]]) == 0)   # API might return a dummy list
      return(NULL)
    #x$data[[1]] <- rationalise10HzColumnNames(x$data[[1]])  # should we process elements > 1?
    return(x)   
  }

  sURLs <- credentials$getURL(sURLs) 
  if (credentials$getTrace())
    print(sURLs)  
  
  # proceed with crul for the multiple athletes case
  credentials$resetApiStatus()
  crul::set_opts(timeout_ms = credentials$getApiTimeout()*1000)
  cc <- crul::Async$new(urls = sURLs,
                        headers = list("Authorization" = credentials$getAuthorizationBearer()) 
                        )
  res <- cc$get()
  #stopifnot(all(vapply(res, function(z) z$success(), logical(1))))
  x <- lapply(res, function(z){
                                res <- safe_validate_crul_http_response(z)
                                if (!is.null(res$error))
                                  return(list(error = paste(res$error, suppressMessages(z$parse()))))
                                if (z$success() != TRUE)    # (status code > 201) && (status code < 300)
                                {
                                  warning(paste(z$status_http()$status_code, z$status_http()$message, z$status_http()$explanation))
                                }
                                sensorData <- jsonlite::fromJSON(z$parse("UTF-8"))
                                if (length(sensorData) == 0)
                                  return(NULL)
                                if (length(sensorData$data) == 0)   # OW-1054: API might return an empty list
                                  return(NULL)
                                if (length(sensorData$data[[1]]) == 0)   # API might return a dummy list
                                  return(NULL)
                                # sensorData - list with "athlete_id", "device_id", "player_id", 
                                # "athlete_first_name", "athlete_last_name","jersey", "team_id", 
                                # "team_name", "stream_type", "data". data[[1]] - dataframe with 10 Hz series 
                                #sensorData$data[[1]] <- rationalise10HzColumnNames(sensorData$data[[1]])
                                return(sensorData)
                              })
  names(x) <- athlete_id
  return(x)
}

rationalise10HzColumnNames<-function(x)
{
  x %>% dplyr::rename(Timestamp=ts) %>% dplyr::rename(Centiseconds=cs) %>% dplyr::rename(Latitude=lat)  %>% dplyr::rename(Longitude=long) %>% 
        dplyr::rename(Velocity=v) %>% dplyr::rename(Acceleration=a) %>% dplyr::rename(Odometer=o) %>% dplyr::rename(PlayerLoad=pl) %>% 
        dplyr::rename(SmoothedPlayerLoad=sl) %>% dplyr::rename(HeartRate=hr) %>% dplyr::rename(MetabolicPower=mp)
  # 'PlayerLoad' - accumulated raw PL; SmoothedPlayerLoad - smoothed PL
}

#' get 10 Hz sensor and positional data for a given athlete and a given activity or period
#'
#' \code{ofCloudGetActivitySensorData} returns the 10 Hz sensor data for a given athlete(s) and a given activity,
#' assuming the athlete is mapped to the activity.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param athlete_id athlete id as returned by \code{\link{ofCloudGetAthletes}}, \code{\link{ofCloudGetAthletesInActivity}}, \code{\link{ofCloudGetAthletesInPeriod}}, 
#'                   \code{\link{ofCloudGetAthleteDevicesInActivity}} or \code{\link{ofCloudGetAthleteDevicesInPeriod}}
#' @param activity_id activity id as returned by \code{\link{ofCloudGetActivities}}
#' @param from,to POSIX time in seconds since the start of the epoch, \code{NA} by default,
#'        can be specified either as \code{(from, to)} or as a half-open interval \code{(from,)} / \code{(,to)}.
#'        If \code{from} is not \code{NA}, then only data with the \code{start time >= from} are returned.
#'        If \code{to} is not \code{NA}, then only data with the \code{end_time <= to} are returned.
#' @param stream_type \code{"gps"} or \code{"lps"}, \code{NA} by default, designating the activity's or period's stream type to return 
#' data for. This parameter is meaningful for dual stream devices only (like VECTOR device), assuming activity's or period's 
#' stream type is valid.
#' @param parameters a vector of character strings with required parameters to return. Valid parameters are\cr
#' \code{"ts"}, \code{"cs"}: POSIX time in seconds since the start of the epoch, observation time offset in centiseconds\cr
#' \code{"lat"}, \code{"long"}, \code{"alt"}: latitude, longitude (degrees), altitude (meters)\cr
#' \code{"xy"}: field \code{x} and \code{y} coordinates (metres)\cr
#' \code{"o"}: odometer (meters)\cr
#' \code{"v"}, \code{"rv"}: velocity, raw velocity (metres per second)\cr
#' \code{"a"}: acceleration (metres per second per second)\cr
#' \code{"hr"}: heart rate (beats per minute)\cr
#' \code{"pl"}, \code{"sl"}: Accumulated Player Load, smoothed load (used for banding PL)\cr
#' \code{"mp"}: metabolic power\cr
#' \code{"pq"}: positional quality (percentage)\cr
#' \code{"ref"}: count of fix references (number of available satellites for GPS, number of anchors/receivers for LPS)\cr
#' \code{"hdop"}: horizontal dilution of precision (applicable to GPS only)\cr
#' \code{"face"}: the magnetic facing of the unit without compensation for declination (degrees)
#' @param field a list with venue details to use in conversion from \code{lat}, \code{long} into field \code{x} and \code{y} coordinates. By default, conversion will use the venue details as available from \code{\link{ofCloudGetActivities}}. Valid entries in the list are\cr
#' \code{centre_latitude}, \code{centre_longitude}: latitude and longitude of the venue,\cr 
#' \code{rotation}: rotation of the venue,\cr
#' \code{length}, \code{width}: length and width of the venue, used to offset the origin of the pitch in the 10 Hz data frame returned.\cr
#' All entries are optional. If supplied, an entry will override the corresponding value as available from \code{\link{ofCloudGetActivities}} or \code{\link{ofCloudGetActivity}}. 
#' @param page_ordinal,page_size page ordinal number of size \emph{page_size} to return. \emph{page_ordinal} starts from 1. \emph{page_size} is in seconds. There could be a time jitters in 10 Hz series, so the number of records in a page might vary.
#' @return \code{ofCloudGetActivitySensorData / ofCloudGetPeriodSensorData / ofCloudGetMixedPeriodsActivitySensorData} returns 
#' either NULL if there are no data to return, or a list with some metadata and the 10 Hz sensor data (as a data frame 
#' in the list element \code{data[[1]]}); \code{stream_type} element of the returned list contains either \code{"gps"} or \code{"lps"}
#' values to indicate the stream type; in the case of \code{ofCloudGetMixedPeriodsActivitySensorData} \code{stream_type} contains 
#' \code{"mixed"} if the requested \code{from / to} interval contains mixed periods. 
#' The function throws if the HTTP response indicates failure. In particular, it throws \strong{404} if data was not synchronised, see an example in \code{\link{ofCloudParseError}}.
#' @examples
#' \dontrun{
#' x <- ofCloudGetActivitySensorData(credentials, athlete_id, activity_id)
#' data10Hz <- x$data[[1]]
#' streamType <- x$stream_type
#' }
#' @family cloud data access APIs
ofCloudGetActivitySensorData<-function(credentials, athlete_id, activity_id, from = NA, to = NA, stream_type = NA, parameters = NA,
                                       field = NULL, page_ordinal = NA, page_size = NA)
{
  ofCloudGetActivityPeriodSensorData(credentials, athlete_id, activity_id, TRUE, from, to, page_ordinal, page_size,
                                     stream_type, parameters, field, FALSE)
}

#' @describeIn ofCloudGetActivitySensorData get 10 Hz sensor and positional data for a given athlete and a given period,
#' assuming the athlete is mapped to the period.
#' @export
#' @param period_id period ID as returned by \code{\link{ofCloudGetPeriods}} or \code{\link{ofCloudGetActivities}}
ofCloudGetPeriodSensorData<-function(credentials, athlete_id, period_id, from = NA, to = NA, stream_type = NA, parameters = NA,
                                     field = NULL, page_ordinal = NA, page_size = NA)
{
  ofCloudGetActivityPeriodSensorData(credentials, athlete_id, period_id, FALSE, from, to, page_ordinal, page_size,
                                     stream_type, parameters, field, FALSE)
}

#' @describeIn ofCloudGetActivitySensorData get 10 Hz sensor and positional data for given athletes and a given activity,
#' assuming the athletes are mapped to the activity. 
#' @export
ofCloudGetActivitySensorDataEx<-function(credentials, athlete_ids, activity_id, from = NA, to = NA, stream_type = NA, parameters = NA,
                                         field = NULL, page_ordinal = NA, page_size = NA)
{ofCloudGetActivityPeriodSensorData(credentials, athlete_ids, activity_id, TRUE, from, to, page_ordinal, page_size,
                                    stream_type, parameters, field, TRUE)}

#' @describeIn ofCloudGetActivitySensorData get 10 Hz sensor and positional data for given athletes and a given period,
#' assuming the athletes are mapped to the period.
#'
#' \code{ofCloudGetActivitySensorDataEx} and \code{ofCloudGetPeriodSensorDataEx} trigger one cloud API asynchronous call for each
#' given athlete without multithreading, outperforming a sequence of 
#' \code{ofCloudGetActivitySensorData / ofCloudGetPeriodSensorData} calls. 
#' However, using \code{dopar} from doParallel  package will achieve better performance, assuming
#' \code{parallel::makeCluster(Number_of_athletes, type='PSOCK')} context.
#'
#' Be mindful not to overload the server with excessive number of requests. Keep the number of athlete IDs passed into
#' \code{ofCloudGetActivitySensorDataEx} and \code{ofCloudGetPeriodSensorDataEx} reasonable. 
#' Otherwise, the requests might be throttled on the server.
#'
#' @export
#' @param athlete_ids a vector of athlete ids
#' @return \code{ofCloudGetActivitySensorDataEx/ofCloudGetPeriodSensorDataEx} returns a list of lists with an entry for each athlete, 
#' with an entry name matching the athlete id, and the entry content matching
#' \code{ofCloudGetActivitySensorData/ofCloudGetPeriodSensorData} return value.
#' The function does not throw if a HTTP call fails for any athlete. \code{error} will be present in the returned list in an entry 
#' for an athlete with a failed call. 
ofCloudGetPeriodSensorDataEx<-function(credentials, athlete_ids, period_id, from = NA, to = NA, stream_type = NA, parameters = NA,
                                       field = NULL, page_ordinal = NA, page_size = NA)
{ofCloudGetActivityPeriodSensorData(credentials, athlete_ids, period_id, FALSE, from, to, page_ordinal, page_size,
                                    stream_type, parameters, field, TRUE)}

#' @describeIn ofCloudGetActivitySensorData get 10 Hz sensor and positional data for a given athlete and a given activity, treating
#' the activity as a mixed (dual stream) activity. A period must be tagged as either \emph{GPS} or \emph{LPS} in console for this feature to work.
#'
#' A mixed (dual stream) activity contains periods from both, \code{gps} and \code{lps} streams. The periods with the stream
#' type different to the default activity stream type will be honoured by the function as long as the athlete is mapped 
#' to the period. If this is the case, the function fetches the positional data relevant to the period's stream type. 
#' As a result, the returned list element \code{data[[1]]} does not guarantee homogeneous 10 Hz time-stamps as returned 
#' in the \code{ts and cs} columns of the \code{data[[1]]}. 
#' @export
ofCloudGetMixedPeriodsActivitySensorData<-function(credentials, athlete_id, activity_id, from = NA, to = NA, parameters = NA, 
                                                   field = NULL)
{
  sdActivity <- ofCloudGetActivitySensorData(credentials, athlete_id, activity_id, from = from, to = to, 
                                             parameters = parameters, field = field)
  if (is.null(sdActivity))
    return(NULL)
  # if (all(parameters %in% c("ts", "cs", "hr", "pl", "face"))) return(sdActivity)  # no positional data requested
  dfData <- sdActivity$data[[1]]

  bMixed <- FALSE
  periods <- ofCloudGetPeriods(credentials, activity_id) 
  for (j in seq_len(NROW(periods)))
  {
    if (!(periods$tag_list[j] %in% c("gps", "lps", "GPS", "LPS", 
                                   stringr::str_to_upper(sdActivity$stream_type), stringr::str_to_lower(sdActivity$stream_type))))
      next    # OW-1776: periods$tag_list[j] is not filled in 
    if (stringr::str_to_upper(sdActivity$stream_type) == stringr::str_to_upper(periods$tag_list[j])) # "GPS" or "LPS"
      next    # same stream type as for activity

    periodFrom <- periods$start_time[j]
    periodTo <- periods$end_time[j]
    if (!is.na(from))
    {
      if (periodTo < from)
        next
      periodFrom <- max(periodFrom, from)
    }
    if (!is.na(to))
    {
      if (periodFrom > to)
        next
      periodTo <- min(periodTo, to)
    }
    
    # is the athlete mapped to the period?
    athletes <- ofCloudGetAthletesInPeriod(credentials, periods$id[j])
    if (athlete_id %in% athletes$id)
    {
      bMixed <- TRUE
      sdPeriod <- ofCloudGetPeriodSensorData(credentials, athlete_id, periods$id[j], from = periodFrom, to = periodTo, 
                                             parameters = parameters, field = field)
      dfData <- as.data.frame(rbind(dfData[dfData$ts < periodFrom, ], sdPeriod$data[[1]], dfData[dfData$ts > periodTo, ]))
    }
  }
  
  if (bMixed)
  {
    sdActivity$data[[1]] <- dfData
    sdActivity$stream_type <- "mixed"
  }
  return(sdActivity)
}

ofCloudGetEvents<-function(credentials, athlete_id, id, events, from=NA, to=NA, ima_acceleration_intensity_threshold = 0, activityPeriod = TRUE)
{
  stopifnot(!is.null(athlete_id) && !is.null(id))
  eventsIma <- events[which(events %in% ima_events())]
  eventsImaEnum <- suppressWarnings(as.numeric(events[which(!(events %in% ima_events()))]))
  stopifnot(!anyNA(eventsImaEnum))
  ima_jump_ml_code <- "20"; 
  ima_jump_ml_count <- length(which(eventsIma=="ima_jump_ml"))
 
  # "ima_jump_ml" is not supported, since the sport plugin was never released;
  # replace "ima_jump_ml" with a numeric code, otherwise the API fails
  eventsIma <- replace(eventsIma, eventsIma=="ima_jump_ml", ima_jump_ml_code) 
  
  e <- paste0(unique(c(eventsIma, eventsImaEnum)), collapse=",")
  sURL <- stringr::str_c("/api/v6/", ifelse(activityPeriod, "activities", "periods"), "/", 
                         id, "/athletes/", athlete_id, "/events?event_types=", e)
  if (!is.na(from) && !is.na(to))
  {
    stopifnot(from < to)
  }
  if (!is.na(from) && (as.integer(from) != 0))    # the endpoint ignores endTime if startTime is 0
  {
    sURL <- stringr::str_c(sURL, "&start_time=", from)
  }
  if (!is.na(to))
  {
    sURL <- stringr::str_c(sURL, "&end_time=", to)
  }
  if (ima_acceleration_intensity_threshold != 0)
  {
    if (!("ima_acceleration" %in% eventsIma) && !(0 %in% eventsImaEnum))
    {
      warning("'ima_acceleration_intensity_threshold' is specified, but 'ima_acceleration' events are not requested")
    }
    else
    {
      ima_threshold <- suppressWarnings(as.numeric(ima_acceleration_intensity_threshold))
      if (is.na(ima_threshold))
        stop("invalid parameter 'ima_acceleration_intensity_threshold'")
      sURL <- stringr::str_c(sURL, "&ima_acceleration_intensity_threshold=", ima_acceleration_intensity_threshold)
    }
  }
  # To get cs as a fractional value in POSIX time, at least 12 digits is needed.
  if (getOption("digits") < 12)
    warning("getOption('digits') should return at least 12 to see centiseconds as a fractional part of the start and end POSIX time for events returned")
  
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  if (is.null(s))   # no events
    return(NULL)
  data <- s$data # disregarding metadata like athlete_id, device_id, player_id, athlete_first_name, athlete_last_name,jersey, team_id and team_name
  
  # return list of dataframes by simplifying the list returned by the above call to fromJSON(). TODO: consider jsonlite::flatten().
  a <- lapply(seq_along(data), function(j)
                              {
                                x <- data[j]
                                if (length(x[[1]]) == 0)
                                  return(NULL)
                                return(x[1,1][[1]])
                              })
  names(a) <- names(data) 
  # "ima_jump_ml" case: replace numbers as a name with the name as a strings
  if (ima_jump_ml_count > 0)
    names(a) <- replace(names(a), names(a)==ima_jump_ml_code, "ima_jump_ml")
  return(a)
}

#' get specified events for the specified athlete and activity
#'
#' \code{ofCloudGetActivityEvents} returns events for a given athlete and a given activity.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param athlete_id athlete id as returned by \code{\link{ofCloudGetAthletes}}, \code{\link{ofCloudGetAthletesInActivity}},
#' \code{\link{ofCloudGetAthletesInPeriod}}, \code{\link{ofCloudGetAthleteDevicesInActivity}} or \code{\link{ofCloudGetAthleteDevicesInPeriod}}
#' @param activity_id activity id as returned by \code{\link{ofCloudGetActivities}}
#' @param events vector of event types, valid vector entries are in \code{\link{ima_events}}; could be specified as numeric IDs (for advanced users)
#' @param from,to POSIX time in seconds since the start of the epoch, \code{NA} by default,
#'        can be specified either as \code{(from, to)} or as a half-open interval \code{(from,)} / \code{(,to)}.
#'        If \code{from} is not \code{NA}, then only events with the \code{start time >= from} are returned.
#'        If \code{to} is not \code{NA}, then only events with the \code{end_time <= to} are returned.
#' @param ima_acceleration_intensity_threshold intensity threshold for \code{'ima_acceleration'} events, default \code{0}
#' @return \code{ofCloudGetActivityEvents} returns the specified events (as a named list of dataframes, one entry per one event type), or \code{NULL} if there are no data to return.
#' @family cloud data access APIs
ofCloudGetActivityEvents<-function(credentials, athlete_id, activity_id, events, from = NA, to = NA, ima_acceleration_intensity_threshold = 0)
{ofCloudGetEvents(credentials, athlete_id, activity_id, events, from, to, ima_acceleration_intensity_threshold, TRUE)}

#' @describeIn ofCloudGetActivityEvents get specified events for the specified athlete and period
#' @export
#' @param period_id period ID as returned by \code{\link{ofCloudGetPeriods}} or \code{\link{ofCloudGetActivities}}
ofCloudGetPeriodEvents<-function(credentials, athlete_id, period_id, events, from = NA, to = NA, ima_acceleration_intensity_threshold = 0)
{ofCloudGetEvents(credentials, athlete_id, period_id, events, from, to, ima_acceleration_intensity_threshold, FALSE)}

ofCloudGetEfforts<-function(credentials, athlete_id, id, velocityOrAcceleration, bands=NA, stream_type=NA, from=NA, to=NA, activityPeriod = TRUE)
{
  stopifnot(!is.null(athlete_id) && !is.null(id))
  
  theModule <- ifelse(velocityOrAcceleration, "Gen2VelocityBands.Released", "Gen2AccelerationBands.Released")
  if (!credentials$validateModules(theModule))
  {
    stop(stringr::str_c(theModule, 
         " module is not present in the account. Please contact Catapult support to enable the module, then reprocess through console and complete a full sync to upload efforts into the cloud."))
  }
  
  sURL <- stringr::str_c("/api/v6/", ifelse(activityPeriod, "activities", "periods"), "/", 
             id, "/athletes/", athlete_id, "/efforts?effort_types=", ifelse(velocityOrAcceleration, c("velocity"), c("acceleration")))
  if ((length(bands) > 1) || !is.na(bands))
  {
    sURL <- stringr::str_c(sURL, ifelse(velocityOrAcceleration, "&velocity_bands=", "&acceleration_bands="), paste0(bands, collapse=","))
  }
  if (!is.na(stream_type))
  {
    sURL <- stringr::str_c(sURL, "&stream_type=", stream_type)    # "gps" or "lps"
  }
  if (!is.na(from) && !is.na(to))
  {
    stopifnot(from < to)
  }
  if (!is.na(from) && (as.integer(from) != 0))    # the endpoint ignores endTime if startTime is 0
  {
    sURL <- stringr::str_c(sURL, "&start_time=", from)
  }
  if (!is.na(to))
  {
    sURL <- stringr::str_c(sURL, "&end_time=", to)
  }
  # To get cs as a fractional value in POSIX time, at least 12 digits is needed.
  if (getOption("digits") < 12)
    warning("getOption('digits') should return at least 12 to see centiseconds as a fractional part of the start and end POSIX time for efforts returned")
  
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  if (is.null(s))   # no efforts
    return(NULL)
    
  data <- s$data # disregarding metadata like athlete_id, device_id, player_id, athlete_first_name, athlete_last_name,jersey, team_id and team_name
  x <- data[1]
  if (length(x[[1]]) == 0)
    return(NULL)
  return(x[1,1][[1]])
}

#' get specified efforts for the specified athlete and activity
#'
#' \code{ofCloudGetActivityEfforts} returns efforts for a given athlete and a given activity. A relevant module (\code{Gen2VelocityBands.Released} or \code{Gen2AccelerationBands.Released}) should be allocated to your account by Catapult support, otherwise respective efforts are not available. Check modules with {\link{ofCloudGetModules}}. Once a module is allocated, reprocess your data through console and complete a full sync to upload efforts into the cloud.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param athlete_id athlete id as returned by \code{\link{ofCloudGetAthletes}}, \code{\link{ofCloudGetAthletesInActivity}}, \code{\link{ofCloudGetAthletesInPeriod}}, 
#'                   \code{\link{ofCloudGetAthleteDevicesInActivity}} or \code{\link{ofCloudGetAthleteDevicesInPeriod}}
#' @param activity_id activity id as returned by \code{\link{ofCloudGetActivities}}
#' @param velocityOrAcceleration \code{TRUE} to request gen. 2 velocity efforts, \code{FALSE} to request gen. 2 acceleration efforts. 
#' @param bands vector of required bands. Valid bands are: 1 to 8 for velocities; -3, -2, -1 for decelaration; 1, 2, 3 for acceleration.
#' @param stream_type the required stream type; the valid types are \code{"gps"} and \code{"lps"}. By default, a stream type mapped to the specified activity or period will be used.
#' @param from,to POSIX time in seconds since the start of the epoch, \code{NA} by default,
#'        can be specified either as \code{(from, to)} or as a half-open interval \code{(from,)} / \code{(,to)}.
#'        If \code{from} is not \code{NA}, then only efforts with the \code{start time >= from} are returned.
#'        If \code{to} is not \code{NA}, then only efforts with the \code{end_time <= to} are returned.
#' @return ofCloudGetActivityEfforts() returns the specified efforts, or \code{NULL} if there are no data to return.
#' @family cloud data access APIs
ofCloudGetActivityEfforts<-function(credentials, athlete_id, activity_id, velocityOrAcceleration, bands=NA, stream_type=NA, from=NA, to=NA)
{ofCloudGetEfforts(credentials, athlete_id, activity_id, velocityOrAcceleration, bands, stream_type, from, to, activityPeriod=TRUE)}

#' @describeIn ofCloudGetActivityEfforts get specified efforts for the specified athlete and period
#' @export
#' @param period_id period ID as returned by \code{\link{ofCloudGetPeriods}} or \code{\link{ofCloudGetActivities}}
ofCloudGetPeriodEfforts<-function(credentials, athlete_id, period_id, velocityOrAcceleration, bands=NA, stream_type=NA, from=NA, to=NA)
{ofCloudGetEfforts(credentials, athlete_id, period_id, velocityOrAcceleration, bands, stream_type, from, to, activityPeriod=FALSE)}

ofCloudGetAthletesInActivityOrPeriod <-function(credentials, id, activityPeriod)
{
  sURL <- stringr::str_c("/api/v6/", ifelse(activityPeriod, "activities", "periods"), "/", id, "/athletes")
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' @describeIn ofCloudGetAthletes get all athletes for a specified activity.\cr
#' \code{\link{ofCloudGetAthleteDevicesInActivity}} returns similar data, and additionally device id.
#'
#' @export
#' @param activity_id activity ID as returned by \code{\link{ofCloudGetActivities}}
#' @return \code{ofCloudGetAthletesInActivity} and \code{ofCloudGetAthletesInPeriod} return a data frame with athletes in the specified activity or period.
#' The functions throw if the HTTP response indicates failure. In particular, it throws \strong{404} if athletes are not mapped to the specified activity or period, see an example in \code{\link{ofCloudParseError}}.
ofCloudGetAthletesInActivity <-function(credentials, activity_id)
{ofCloudGetAthletesInActivityOrPeriod(credentials, activity_id, activityPeriod=TRUE)}

#' @describeIn ofCloudGetAthletes get all athletes for a specified period.\cr
#' \code{\link{ofCloudGetAthleteDevicesInPeriod}} returns similar data, and additionally device id.
#'
#' @export
#' @param period_id period ID as returned by \code{\link{ofCloudGetPeriods}} or \code{\link{ofCloudGetActivities}}
ofCloudGetAthletesInPeriod <-function(credentials, period_id)
{ofCloudGetAthletesInActivityOrPeriod(credentials, period_id, activityPeriod=FALSE)}

#' return the specified statistics
#'
#' Returns a data frame with the specified statistics. Use \code{\link{ofCloudGetUserSettings}} to retrieve velocity and distance units.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param params parameters to return, like \code{'total_player_load'}, \code{'total_duration'}, \code{'total_distance'} etc.\cr
#' All available parameters are returned by \code{\link{ofCloudGetParameters}}.
#' @param groupby how to group the results, an element from a set \code{'athlete', 'activity', 'period', 'position', 'team'},\cr
#' or any combination of the elements. \code{'athlete'} by default. If \code{annotationStats == TRUE} then \code{'annotation_category', 'annotation'} are also valid \code{groupby} values.
#' @param filters a list with one or more filters. Each filter is a list with three elements,\cr
#' 1. \code{name} - a field to filter, from the set \code{'date', 'period_id', 'activity_id', 'athlete_id', 'position_id', 'team_id', 'athlete_group_id', 'lastActivities', 'timeRange', 'period_name', 'snapshot_name', 'rotationNumber', 'tag_id', 'month_id', 'day_id'},\cr
#' 2. \code{comparison} - what type of filter to use \code{(=, <>, >, <, >=, <=)},\cr
#' 3. \code{values} - values for the filter, \emph{e.g.} an activity ID(s) for the \code{'activity_id'} filter.
#' @param annotationStats query annotation database, default \code{FALSE}.\cr
#' To group by annotation_category and/or annotation (annotation name) annotationStats must be \code{TRUE}.
#' @return \code{ofCloudGetStatistics} returns a data frame with the specified statistics, or \code{NULL} if there are no rows to return. 
#' @examples
#' \dontrun{
#' activities <- ofCloudGetActivities(token)
#' grouping <- c("athlete", "activity", "period")
#' parameters <- c("total_player_load", "total_distance")
#' filter1 <- list("name"="activity_id", "comparison" = "=", "values" = c(id1, id2))
#' filter2 <- list("name"="athlete_name", "comparison" = "=", "values" = "Athlete Name")
#' df1 <- ofCloudGetStatistics(credentials = token, params = parameters, groupby = grouping, filters = filter1) 
#' df2 <- ofCloudGetStatistics(credentials = token, params = parameters, groupby = grouping, filters = filter2) 
#' df3 <- ofCloudGetStatistics(credentials = token, params = parameters, groupby = grouping, filters = c(filter1, filter2)) 
#' }
#' @family cloud metadata access APIs
ofCloudGetStatistics<-function(credentials, params, groupby="athlete", filters, annotationStats = FALSE)
{
  stopifnot(length(filters)%%3 == 0)
  allGroupby <- c("athlete", "activity", "period", "position", "team")
  if (annotationStats)
	allGroupby <- c(allGroupby, "annotation_category", "annotation")
  stopifnot(all(groupby %in% allGroupby))
  
  filter_df <- data.frame(matrix(ncol = 3, nrow = (length(filters)/3)), stringsAsFactors = FALSE)
  keys <- c("name", "comparison", "values")
  colnames(filter_df) <- keys
  fNames <- names(filters)
  stopifnot(all(fNames %in% keys))
  sValues <- c()
  for (j in seq_len(NROW(filter_df)))
  {
    filterJ <- list(filters[[j*3-2]], filters[[j*3-1]], filters[[j*3]])
    names(filterJ) = c(fNames[j*3-2], fNames[j*3-1], fNames[j*3])
    stopifnot(!is.null(filterJ$name)); stopifnot(!is.null(filterJ$comparison)); stopifnot(!is.null(filterJ$values))
    stopifnot(filterJ$name %in% c("date", "period_id", "activity_id", "athlete_id", "position_id", "team_id", "athlete_group_id",
                                  "lastActivities", "timeRange", "period_name", "snapshot_name", "rotationNumber", "tag_id",
                                  "month_id", "day_id"))
    #stopifnot(filterJ$comparison %in% c("=", "<>", ">", "<", ">=", "<=")) # cloud validates the operation
    filter_df$name[j] <- filterJ$name
    filter_df$comparison[j] <- filterJ$comparison
    filter_df$values[j] <- 1234567890  # placeholder, could not find a more elegant solution (knitr is more elegant, but it would be an additional dependency)
    sCompValue <- paste(stringr::str_c('"', filterJ$values, '"'), collapse=",")
    sCompValue <- stringr::str_c("[", sCompValue, "]")
    sValues <- c(sValues, sCompValue)   # TODO: escapeForJSON(sCompValue)
  }
  
  lst <- list(filters = filter_df,
              parameters = params,
              group_by = groupby)
  sBody <- jsonlite::toJSON(lst)

  for (j in seq_len(NROW(filter_df)))
  {
    sBody <- stringr::str_replace(sBody, '1234567890', sValues[j])
  }

  sURL <- "/api/v6/stats?requested_only=TRUE"
  if (annotationStats) 
    sURL <- stringr::str_c(sURL, "&source=annotation_stats") 
  return(credentials$httrPost(sURL, sBody))
}

ofCloudGetModulesImp<-function(credentials)
{
  sURL <- "/api/v6/modules"
  rawContent <- credentials$httrGet(sURL, contentType = "raw")  # httrGet() calls stop() if (credentials$apiStatus != 200)
  
  # this endpoint returns HTML
  htmlContent <- suppressWarnings(xml2::read_xml(rawContent, as_html=TRUE))
  moduleNodes <- xml2::xml_find_all(htmlContent, "//modules")
  modules <- xml2::xml_children(moduleNodes)
  ret <- sapply(seq_len(length(modules)), function(j){xml2::xml_text(modules[j])})
  return(ret)
}

#' return the modules allocated to the account, like \code{'HighFrequencyData'} \emph{etc.}
#'
#' Returns a data frame with modules.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetModules} returns a data frame with modules. 
#' @family cloud metadata access APIs
ofCloudGetModules<-function(credentials)
{
  credentials$validateModules(NULL, TRUE)
  return(credentials$getModules())		# note, apiScope:catapultR module is required for OF Cloud APIs to work through R
}

#' get bands for a given athlete
#'
#' ofCloudGetBands() returns a data frame with bands.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param athlete_id athlete id as returned by \code{\link{ofCloudGetAthletes}}, \code{\link{ofCloudGetAthletesInActivity}}, \code{\link{ofCloudGetAthletesInPeriod}}, 
#' \code{\link{ofCloudGetAthleteDevicesInActivity}} or \code{\link{ofCloudGetAthleteDevicesInPeriod}}.
#' @return \code{ofCloudGetBands} returns a data frame with bands. 
#' @family cloud metadata access APIs
ofCloudGetBands <- function(credentials, athlete_id)
{
  sURL <- stringr::str_c("/api/v6/athletes/", athlete_id, "/bands")
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' @describeIn ofCloudGetBands get band details for a given athlete and a band
#' @export
#' @param sBand a band name as available from the data frame returned by \code{\link{ofCloudGetBands}}, \emph{e.g.} \code{'Gen2Acceleration'}, \code{'Velocity2'}
#' @return \code{ofCloudGetBands} returns a data frame with band details.\cr
#' The rows with \code{end_time} attribute equal \code{0} correspond to the current (as opposite to historical) band.
ofCloudGetBand <- function(credentials, athlete_id, sBand)
{
  sURL <- stringr::str_c("/api/v6/athletes/", athlete_id, "/bands?band_name=", sBand)
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' get teams on the account
#'
#' Returns a data frame with teams.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetTeams} returns a data frame with teams. 
#' @family cloud metadata access APIs
ofCloudGetTeams <- function(credentials)
{
  return(credentials$httrGet("/api/v6/teams"))
}

#' get customer info for the account
#'
#' Returns a data frame with customer information.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetCustomerInfo} returns a data frame with customer information. 
#' @family cloud metadata access APIs
ofCloudGetCustomerInfo <- function(credentials)
{
  s <- credentials$httrGet("/api/v6/customer/info")  # httrGet() calls stop() if (credentials$apiStatus != 200)
  if (is.null(s))
    return(NULL)
  return(as.data.frame(t(unlist(s)), stringsAsFactors = FALSE))
}

#' get venues
#'
#' \code{ofCloudGetVenues} returns a data frame with venues system attributes.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetVenues} returns a data frame with venues system attributes. 
#' @family cloud metadata access APIs
ofCloudGetVenues<-function(credentials)
{
  s <- credentials$httrGet("/api/v6/venues")  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(s)
}

#' @describeIn ofCloudGetVenues return a data frame with a given venue system attributes.
#' @export
#' @param id a venue id as available from \code{ofCloudGetVenues}
#' @return \code{ofCloudGetVenue} returns a data frame with a given venue system attributes.
ofCloudGetVenue<-function(credentials, id)
{
  sURL <- stringr::str_c("/api/v6/venues/", id)
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(as.data.frame(s, stringsAsFactors = FALSE))
}

#' get activity annotations
#'
#' \code{ofCloudGetActivityAnnotations} returns a data frame with activity annotations.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @param activity_id activity id as returned by \code{\link{ofCloudGetActivities}}
#' @return \code{ofCloudGetActivityAnnotations} returns a data frame with activity annotations, including \emph{comment} and \emph{annotation category}. 
ofCloudGetActivityAnnotations<-function(credentials, activity_id)
{
  sURL <- stringr::str_c("/api/v6/activities/", activity_id, "/annotations")
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(as.data.frame(s, stringsAsFactors = FALSE))
}

#' @describeIn ofCloudGetActivityAnnotations returns a data frame with period annotations.
#' @export
#' @param period_id period ID as returned by \code{\link{ofCloudGetPeriods}} or \code{\link{ofCloudGetActivities}}
#' @return \code{ofCloudGetPeriodAnnotations} returns a data frame with period annotations, including \emph{comment} and \emph{annotation category}. 
ofCloudGetPeriodAnnotations<-function(credentials, period_id)
{
  sURL <- stringr::str_c("/api/v6/periods/", period_id, "/annotations")
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(as.data.frame(s, stringsAsFactors = FALSE))
}

#' @describeIn ofCloudGetActivityAnnotations returns a data frame with athlete annotations.
#' @export
#' @param athlete_id athlete id as returned by \code{\link{ofCloudGetAthletes}}, \code{\link{ofCloudGetAthletesInActivity}}, \code{\link{ofCloudGetAthletesInPeriod}}, \code{\link{ofCloudGetAthleteDevicesInActivity}} or \code{\link{ofCloudGetAthleteDevicesInPeriod}}
#' @param annotation_id as returned by \code{ofCloudGetAthleteAnnotations}
#' @return \code{ofCloudGetAthleteAnnotations} returns a data frame with athlete annotations, including \emph{comment} and \emph{annotation category}. If \emph{annotation_id} is specified then the data frame returned contains details for the given annotation only. 
ofCloudGetAthleteAnnotations<-function(credentials, athlete_id, annotation_id = NULL)
{
  sURL <- stringr::str_c("/api/v6/athletes/", athlete_id, "/annotations")
  if (!is.null(annotation_id))
    sURL <- stringr::str_c(sURL, "/", annotation_id)
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  if (!("data.frame" %in% class(s))) # prevent 'arguments imply differing number of rows: 1, 0' error; JSON returned a single element, not an array (enclosed into square brakets [])
  {
	s <- jsonlite::fromJSON(paste0("[", jsonlite::toJSON(s), "]"))		# enforce JSON array
  }
  return(as.data.frame(s, stringsAsFactors = FALSE))
}

#' @describeIn ofCloudGetActivityAnnotations returns a data frame with annotation categories.
#' @export
#' @return \code{ofCloudGetAnnotationCategories} returns a data frame with annotation categories. 
ofCloudGetAnnotationCategories<-function(credentials)
{
  sURL <- "/api/v6/annotation_categories"
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(as.data.frame(s, stringsAsFactors = FALSE))
}

#' get user settings
#'
#' \code{ofCloudGetUserSettings} returns a data frame with user settings; the settings control the units of velocity and distance as retrieved by \code{\link{ofCloudGetStatistics}}.
#'
#' @export
#' @param credentials as returned by \code{\link{ofCloudGetToken}}, \code{\link{safe_ofCloudGetToken}} or \code{\link{safe_ofCloudGetTokenEx}}
#' @return \code{ofCloudGetUserSettings} returns a data frame with user settings. 
#' @family cloud metadata access APIs
ofCloudGetUserSettings<-function(credentials)
{
  sURL <- "/api/v6/settings"
  s <- credentials$httrGet(sURL)  # httrGet() calls stop() if (credentials$apiStatus != 200)
  return(as.data.frame(s, stringsAsFactors = FALSE))
}

#' parse an error from a failed cloud API
#'
#' \code{ofCloudParseError} is useful to distinguish between failures in the cloud APIs. Typically, a status code \strong{500}
#' is returned if there is a general failure. A status code \strong{404} is returned if data is not found, for example 
#' \code{ofCloudGetActivitySensorData} or \code{ofCloudGetPeriodSensorData} fails with 404 if data was not synchronised from
#' OpenField, or \code{ofCloudGetAthletesInActivity} or \code{ofCloudGetAthletesInPeriod} fails with 404 if athletes are not mapped 
#' to the specified activity or period.\cr
#' This function supplements \code{ofCredentials$getApiStatus()} and \code{ofCredentials$getApiMessage()} functions
#'
#' @export
#' @param err a error as caught by \code{trycatch} or with \code{purrr::safely()}
#' @return \code{ofCloudParseError} returns a list with \code{status_code} and \code{message}. 
#' @examples
#'  # example 1 - a 'safe' function already provided by the package 
#'  result <- safe_ofCloudGetToken("APAC", "me", "mypassword", "main", "x", "y")
#'  if (!is.null(result$error))
#'  {
#'    ofCloudParseError(result$error)
#'    # alternatively, the status and message are available from the following,
#'    credentials <- result$result
#'    print(credentials$getApiStatus())
#'    print(credentials$getApiMessage())
#'  }
#'  # example 2 - using purrr::safely() to derive a 'safe' function 
#'  f <- purrr::safely(ofCloudGetAthletesInActivity)
#'  result <- f(credentials, id)
#'  # proceed as in the example 1 above
ofCloudParseError <- function(err)
{
  if (typeof(err) == "list") {
    err <- err[[1]]
  }
  key <- "status_code = "
  x <- suppressWarnings(unlist(stringr::str_split(err, key, n = 2)))
  if (length(x) <= 1)
    return(list(message = err))
  err <- x[2] 
  x <- unlist(stringr::str_split(err, "\\.", n = 2))
  return(list(status_code = x[1], message = x[2]))
}


         
