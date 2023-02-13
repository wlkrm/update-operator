go mod edit -module github.com/wlkrm/update-controller

find . -type f -name '*.go' \
  -exec sed -i -e 's,update-controller,github.com/wlkrm/update-controller.git,g' {} \;