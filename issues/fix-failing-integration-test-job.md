### Issue: Fix Failing Integration Test Job

The NATS service container (nats:alpine) is marked as unhealthy and fails to start during CI. 

### Proposed Solution Steps:
1. Review and update healthcheck settings in `.github/workflows/integration-test.yml`
2. Increase wait time before health checks
3. Test health checks locally

### References:
- Job logs and workflow at ref `c911b869febbd61356ad661bd25f828a71f8b9fb`