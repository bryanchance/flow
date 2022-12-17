# Kubernetes

The following configs should get an initial Flow system deployed to Kubernetes.

Once deployed, you should see a service called `flow`. You can use a port forward
to access:

```
kubectl port-forward service/flow 7080:7080
```

In another terminal, login with `fctl`:

```
fctl login
```

To deploy workflow processors, first create a service token:

```
fctl accounts generate-service-token
```

That should output a token such as `95e6db1dfff5209dd404df18fe45e5af3e6007c80a83a5b67c5bf320b445ca74`.

Next, create the Kubernetes secret using the token:

```
kubectl create secret generic flow-service-token --from-literal=token=<YOUR-TOKEN>
```

Now you can deploy the workflow processors:

```
kubectl apply -f workflow-processors.yaml
```
