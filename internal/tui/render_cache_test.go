package tui

import "testing"

func TestRenderBodyWithDimensionsCachesStableFrames(t *testing.T) {
	model := benchmarkModel(overviewSection)
	sidebarWidth, bodyHeight, mainWidth := model.bodyDimensions()

	first := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if first == "" {
		t.Fatal("expected initial body render output")
	}
	if model.renderCache == nil || model.renderCache.bodyKey == "" {
		t.Fatalf("expected body cache to be populated, got %+v", model.renderCache)
	}

	model.renderCache.bodyValue = "cached body"
	cached := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if cached != "cached body" {
		t.Fatalf("expected cached body render, got %q", cached)
	}

	model.contentVersion++
	updated := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if updated == "cached body" {
		t.Fatalf("expected content version change to invalidate body cache, got %q", updated)
	}
}

func TestRenderBodyWithDimensionsWithoutRenderCache(t *testing.T) {
	model := benchmarkModel(servicesSection)
	model.renderCache = nil

	sidebarWidth, bodyHeight, mainWidth := model.bodyDimensions()
	rendered := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if rendered == "" {
		t.Fatal("expected body render without cache")
	}
}
