####################
# Common functions #
####################

RED=""
CYAN=""
NOCOLOR=""
setcolor() {
  RED='\033[31;49m'
  CYAN='\033[36;49m'
  NOCOLOR='\033[39;49m'  
}

# gitver returns the current git commit short hash
gitver() {
  git rev-parse --short HEAD
}

# expectKubeContext - returns true if the current kube-context match the first argument
# passed to this function
expectKubeContext() {
    want=$1
    got=$(kubectl config current-context)    
    if [ "${want}" != "${got}" ]; then
      echo -e "${RED}ERROR${NOCOLOR} Invalid kubectl-context:"
      echo -e "    got: ${got}"
      echo -e "   want: ${want}"      	
      exit 121
    fi 
    
    true
}

# everythingIsCommitted - fail if there are uncommitted changes in the repo
everythingIsCommitted() {
  nDiff=$(git diff -w "@{upstream}" | wc -l | tr -d ' ')
  if [[ ${nDiff} -ne 0  ]]; then
    echo -e "${RED}ERROR${NOCOLOR} Please commit and push (or discard) your changes before deploying:"
    git diff -w "@{upstream}"
    exit 122
  fi
  
  true
}

# log a message to stdout with a datetime and colors, if `setcolor` has been invoked
log() {
  _stack=$(caller)
  callerScript=$(echo "${_stack}" | awk -F " " '{print $2}')
  callerLine=$(echo "${_stack}" | awk -F " " '{print $1}')
  callerfile=$(basename ${callerScript})
  echo -e "${CYAN}${callerfile}:${callerLine} $(date +%FT%T)${NOCOLOR} | $*"
}

successLog() {
  echo -e "${CYAN}SUCCESS: ${NOCOLOR} | $*"
}

errorLog() {
  echo -e "${RED}ERROR: ${NOCOLOR} | $*"
}

pauseline() {
  log "$*"
  log "\t\thit any key to continue, ctrl+c to abort"
  # shellcheck disable=SC2034
  # shellcheck disable=SC2162
  IFS= read my_var
}

h1() {
  # Variables
  message=$1
  fixed_length=38  # Total length of the frame
  
  # Calculate padding
  content="${message}"
  content_length=${#content}
  padding=$((fixed_length - content_length - 2))  # Subtract 2 for the spaces before and after content
  left_padding=0
  right_padding=0
  
  if ((padding > 0)); then
      left_padding=$((padding / 2))
      right_padding=$((padding - left_padding))
  fi
  
  # Build the frame
  top_bottom_border=$(printf '#%.0s' $(seq 1 $fixed_length))
  padded_content="#$(printf ' %.0s' $(seq 1 $left_padding))${content}$(printf ' %.0s' $(seq 1 $right_padding))#"
  
  # Output
  log "$top_bottom_border"
  log "$padded_content"
  log "$top_bottom_border"
}

# mustVar extract a value from commandline options, and panic if the variable is not set
# to save the value of the option `--environment` in the variable `env`:
# env=$(mustVar environment $@)
mustVar() {
  varname=$1
  shift
  varvalue=${!varname}
  
  
  if [ -z "$varvalue" ]; then    
    echo -e "${RED}ERROR${NOCOLOR} missing required parameter --${varname}"
    exit 123
  fi

  # if there are no more args, then ok
  if [ "$#" -eq 0 ]; then
      return 0
  fi
  
  # if there are more arguments, test the var value for exact matches
  for var in "$@"
  do
      if [[ "${varvalue}" == "$var" ]]; then
        return 0
      fi
  done

  exit 124

}

increment_semver() {
    local semver=$1

    # Extract the major, minor, and patch numbers
    IFS='.' read -r major minor patch <<< "$semver"

    # Increment the patch number
    patch=$((patch + 1))

    # Output the new version
    echo "$major.$minor.$patch"
}

notify_deploy() {
  local service=$1
  local env=$2
  local tag=$3
  
  icon=":fire:"
  color="#e23a08"
  
  if [[ "$env" == "hack" ]]; then 
    icon=":rainbow:"
    color="#36a64f"      
  fi
  
  deployer="shMakefile / ${USER}"
  if [ "${GITHUB_ACTIONS}" == "true" ]; then
    deployer="<$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID|CICD Pipeline>"
  fi
  
  echo "notify deploy of [${service}] to [${env}] by [${deployer}]"
  
  message=$(cat <<EOM
  {    
    "attachments": 
    [
        {
            "mrkdwn_in": ["text"],
            "color": "${color}",
            "pretext": " - ",
            "title": "${icon} [${env}] A new deployment of [${service}] is ongoing ${icon}",
            "title_link": "https://github.com/Screevosrl/talk-to-me/releases/tag/${tag}",
            "text": " ",
            "fields": 
            [
                {
                    "title": "Environment",
                    "value": "heero-${env}",
                    "short": false
                },
                {
                    "title": "Service",
                    "value": "${service}",
                    "short": true
                },
                {
                    "title": "Who is deploying",
                    "value": "${deployer}",
                    "short": true
                }
            ]
        }
    ]
}

EOM
)
  
  
}