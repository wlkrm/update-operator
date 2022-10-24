kubectl logs $(kubectl get pods -n update-operator-system | tail -n1 | awk '{print $1;}') -n update-operator-system
