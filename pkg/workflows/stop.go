package workflows

func (h *WorkflowHandler) Stop() error {
	h.stopCh <- true
	return nil
}
