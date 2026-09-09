package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"papertrail/internal/document"
	"papertrail/internal/search"
)

type App struct {
	search *search.Service
	ctx    context.Context
	cancel context.CancelFunc
}

func New(
	searchService *search.Service,
	ctx context.Context,
	cancel context.CancelFunc,
) *App {
	return &App{
		search: searchService,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (a *App) Run() {
	application := app.NewWithID("papertrail.demo")
	window := application.NewWindow("PaperTrail")
	window.Resize(fyne.NewSize(1200, 760))
	window.SetMaster()

	window.SetOnClosed(func() {
		a.cancel()
	})

	go func() {
		<-a.ctx.Done()

		fyne.Do(func() {
			application.Quit()
		})
	}()

	queryEntry := widget.NewEntry()
	queryEntry.SetPlaceHolder("Search")

	searchBy := widget.NewSelect([]string{
		"Filename",
		"Date",
		"Author",
		"Creator",
		"Text",
		"Type",
	}, nil)
	searchBy.SetSelected("Text")

	searchButton := widget.NewButton("Search", nil)

	var results []search.SearchResult

	// Incremented for every search.
	// Older search responses are ignored.
	var searchGeneration atomic.Uint64

	detail := container.NewVScroll(
		container.NewVBox(
			boldLabel("Select a document"),
		),
	)

	resultList := widget.NewList(
		func() int {
			return len(results)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel(""),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(results) {
				return
			}

			item := obj.(*fyne.Container)
			title := item.Objects[0].(*widget.Label)
			meta := item.Objects[1].(*widget.Label)

			doc := results[id].Document

			title.SetText(documentTitle(doc))
			meta.SetText(fmt.Sprintf(
				"%s | %s | %s",
				doc.FileMetadata.Filename,
				doc.DocumentMetadata.DocumentType,
				doc.FileMetadata.ModifiedAt.Format("02 Jan 2006"),
			))
		},
	)

	resultList.OnSelected = func(index int) {
		if index < 0 || index >= len(results) {
			return
		}

		result := results[index]

		detail.Content = container.NewVBox(
			boldLabel("Loading..."),
		)
		detail.Refresh()

		go func() {
			documentDetail, err := a.search.GetDocumentDetail(
				a.ctx,
				result.Document.ID,
			)

			if err != nil {
				fyne.Do(func() {
					if a.ctx.Err() != nil {
						return
					}

					detail.Content = container.NewVBox(
						widget.NewLabel(err.Error()),
					)
					detail.Refresh()
				})
				return
			}

			related := make([]relatedDocument, 0, len(documentDetail.Relationships))

			for _, relationship := range documentDetail.Relationships {
				otherID := relationship.DocumentB

				if otherID == documentDetail.Document.ID {
					otherID = relationship.DocumentA
				}

				other, err := a.search.GetDocumentDetail(
					a.ctx,
					otherID,
				)
				if err != nil {
					continue
				}

				related = append(related, relatedDocument{
					Title: documentTitle(other.Document),
					Type:  relationship.RelationshipType,
				})
			}

			if a.ctx.Err() != nil {
				return
			}

			fyne.Do(func() {
				if a.ctx.Err() != nil {
					return
				}

				detail.Content = renderDetail(
					documentDetail,
					result.MatchedEntities,
					related,
				)
				detail.Refresh()
			})
		}()
	}

	runSearch := func() {
		value := strings.TrimSpace(queryEntry.Text)
		if value == "" {
			return
		}

		// Clear previous results immediately.
		results = nil
		resultList.Refresh()

		detail.Content = container.NewVBox(
			widget.NewLabel("Searching..."),
		)
		detail.Refresh()

		generation := searchGeneration.Add(1)

		query := search.SearchQuery{
			By:    searchByValue(searchBy.Selected),
			Value: value,
		}

		go func() {
			found, err := a.search.Search(a.ctx, query)

			if a.ctx.Err() != nil {
				return
			}

			fyne.Do(func() {
				if a.ctx.Err() != nil {
					return
				}

				// Ignore an older search response.
				if generation != searchGeneration.Load() {
					return
				}

				if err != nil {
					results = nil
					resultList.Refresh()

					detail.Content = container.NewVBox(
						widget.NewLabel(err.Error()),
					)
					detail.Refresh()

					return
				}

				if len(found) == 0 {
					results = nil
					resultList.Refresh()

					detail.Content = container.NewVBox(
						widget.NewLabel("No results"),
					)
					detail.Refresh()

					return
				}

				results = found
				resultList.Refresh()

				resultList.Select(0)
			})
		}()
	}

	searchButton.OnTapped = runSearch

	queryEntry.OnSubmitted = func(string) {
		runSearch()
	}

	searchBar := container.NewBorder(
		nil,
		nil,
		nil,
		searchButton,
		container.NewBorder(
			nil,
			nil,
			nil,
			searchBy,
			queryEntry,
		),
	)

	left := container.NewBorder(
		searchBar,
		nil,
		nil,
		nil,
		resultList,
	)

	main := container.NewHSplit(left, detail)
	main.SetOffset(0.38)

	window.SetContent(container.NewPadded(main))
	window.Show()

	application.Run()
}

type relatedDocument struct {
	Title string
	Type  string
}

func renderDetail(
	detail *search.DocumentDetail,
	matched []document.Entity,
	related []relatedDocument,
) fyne.CanvasObject {
	objects := []fyne.CanvasObject{
		boldLabel(documentTitle(detail.Document)),
		widget.NewSeparator(),
		widget.NewLabel(fmt.Sprintf(
			"Type\t%s",
			detail.Document.DocumentMetadata.DocumentType,
		)),
		widget.NewLabel(fmt.Sprintf(
			"Date\t%s",
			detail.Document.FileMetadata.ModifiedAt.Format("02 Jan 2006"),
		)),
		widget.NewLabel(fmt.Sprintf(
			"Path\t%s",
			detail.Document.FileMetadata.SourcePath,
		)),
		widget.NewLabel(fmt.Sprintf(
			"Size\t%s",
			formatSize(detail.Document.FileMetadata.Size),
		)),
		widget.NewLabel(fmt.Sprintf(
			"Filename\t%s",
			detail.Document.FileMetadata.Filename,
		)),
	}

	if detail.Document.FileMetadata.Author != "" {
		objects = append(
			objects,
			widget.NewLabel(fmt.Sprintf(
				"Author\t%s",
				detail.Document.FileMetadata.Author,
			)),
		)
	}

	if detail.Document.FileMetadata.Creator != "" {
		objects = append(
			objects,
			widget.NewLabel(fmt.Sprintf(
				"Creator\t%s",
				detail.Document.FileMetadata.Creator,
			)),
		)
	}

	objects = append(
		objects,
		widget.NewSeparator(),
		boldLabel("Extracted Information"),
	)

	entities := detail.Entities

	if len(matched) > 0 {
		entities = matched
	}

	if len(entities) == 0 {
		objects = append(
			objects,
			widget.NewLabel("No entities"),
		)
	} else {
		for _, entity := range entities {
			objects = append(
				objects,
				widget.NewLabel(
					fmt.Sprintf(
						"%s (%s)",
						entity.Value,
						entity.Type,
					),
				),
			)
		}
	}

	objects = append(
		objects,
		widget.NewSeparator(),
		boldLabel("Related Documents"),
	)

	if len(related) == 0 {
		objects = append(
			objects,
			widget.NewLabel("No related documents"),
		)
	} else {
		for _, item := range related {
			objects = append(
				objects,
				widget.NewLabel(
					fmt.Sprintf(
						"%s  |  %s",
						item.Title,
						item.Type,
					),
				),
			)
		}
	}

	return container.NewVBox(objects...)
}

func searchByValue(value string) search.SearchBy {
	switch value {
	case "Filename":
		return search.SearchByFilename
	case "Date":
		return search.SearchByDate
	case "Author":
		return search.SearchByAuthor
	case "Creator":
		return search.SearchByCreator
	case "Type":
		return search.SearchByType
	default:
		return search.SearchByText
	}
}

func boldLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

func documentTitle(doc document.Document) string {
	if doc.DocumentMetadata.Title != "" {
		return doc.DocumentMetadata.Title
	}

	return filepath.Base(doc.FileMetadata.Filename)
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	return fmt.Sprintf("%.0f KB", float64(size)/1024)
}
