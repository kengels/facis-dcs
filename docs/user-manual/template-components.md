# Template-Komponenten registrieren, veröffentlichen und verwenden

Template-Komponenten sind wiederverwendbare Bausteine für Vertragstemplates.
Nach Prüfung und Freigabe können sie wie andere Templates registriert,
veröffentlicht und anschließend im Template Builder ausgewählt werden.

## Voraussetzungen und Rollen

- Zum Registrieren und Veröffentlichen benötigen Sie die Rolle **Template Manager**.
- Die Komponente muss zum Registrieren den Status **Approved** haben.
- Zum Veröffentlichen muss die Komponente den Status **Registered** haben.
- Zum Einfügen in ein Vertragstemplate benötigen Sie die Rolle **Template Creator**.
- Für den Policy-Audit benötigen Sie eine dafür freigegebene Rolle. Dazu zählen
  **Auditor**, **Compliance Officer** sowie die Rollen des Template-Workflows.

## Komponente registrieren und veröffentlichen

1. Öffnen Sie die Template-Übersicht und suchen Sie die freigegebene Komponente.
2. Wählen Sie **Register**. Die Komponente erhält den Status **Registered**.
3. Wählen Sie anschließend **Publish**.
4. Bestätigen Sie die Veröffentlichung. Die Komponente erhält den Status
   **Published** und bleibt im Katalog als Komponente erkennbar.

## Komponente in einem Vertragstemplate verwenden

1. Öffnen Sie den Template Builder und erstellen Sie ein Vertragstemplate.
2. Öffnen Sie die Auswahl zum Hinzufügen einer Komponente.
3. Wählen Sie die gewünschte Komponente anhand ihres Namens und ihrer DID aus.
4. Fügen Sie die Komponente hinzu. Der Builder lädt die vollständigen aktuellen
   Daten der Komponente und zeigt sie in der Template-Hierarchie an.
5. Speichern Sie den neuen Entwurf. Beim erneuten Öffnen bleibt die Komponente
   mit der beim Auswählen geladenen Version und ihren Daten erhalten.

Die Auswahl zeigt Komponenten mit Status **Registered** oder **Published**.
Der gespeicherte Snapshot macht nachvollziehbar, welche Fassung der Komponente
dem Entwurf zugrunde lag; spätere Änderungen der Komponente überschreiben ihn
nicht stillschweigend.

## Zusammengesetztes Vertragstemplate prüfen

Ein Policy-Audit bewertet ein Vertragstemplate einschließlich der unmittelbar
eingebetteten, gespeicherten Component-Snapshots. Dadurch kann beispielsweise
eine gültige **Data Requirement** oder eine daran gebundene Klausel aus einer
Komponente die entsprechende Anforderung des gesamten Vertragstemplates
erfüllen.

1. Öffnen Sie das gewünschte Vertragstemplate.
2. Wechseln Sie zu **Audit History**. Der Policy-Audit wird beim Öffnen dieser
   Ansicht geladen.
3. Prüfen Sie die angezeigten Findings anhand von Schweregrad, Titel, Meldung
   und Regelreferenz.

Für den gemeinsamen Vertragsinhalt betrachtet das Audit den Parent und seine
unmittelbar gespeicherten Komponenten zusammen. Dies gilt für:

- Data Requirements,
- Klauseln und Bindings,
- Policies,
- Domain Fields und Constraints sowie
- verpflichtende Domain Fields.

Ungültiger Inhalt wird nicht durch gültigen Inhalt an anderer Stelle verdeckt.
Enthält beispielsweise der Parent eine fehlerhafte Data Requirement, bleibt das
ein Finding, selbst wenn eine Komponente zusätzlich eine gültige Data
Requirement bereitstellt. Umgekehrt wird auch fehlerhafter Component-Inhalt
gemeldet.

Struktur, Metadaten und Lifecycle-Status des Parent-Templates werden weiterhin
nur am Parent geprüft. Eine fehlerhafte interne Layout-Struktur einer
Komponente ersetzt daher nicht die Strukturprüfung des Parent-Templates.

Das Audit verwendet genau die beim Speichern eingebettete Komponentenfassung.
Es lädt weder eine inzwischen geänderte Fassung aus dem Repository nach, noch
verändert es den gespeicherten Snapshot. In einer eingebetteten Komponente
enthaltene weitere Snapshots werden nicht rekursiv ausgewertet. Damit bleibt das
Prüfergebnis für die gespeicherte Vertragsvorlage reproduzierbar.

Bei Component-Findings bleiben Meldung und Regelreferenz in der Audit History
sichtbar. Der technische Quellpfad im Audit-Ergebnis beginnt bei solchen
Findings mit dem betroffenen Snapshot, zum Beispiel
`dcs:metadata.dcs:subTemplates[0].dcs:template`. Dieser technische Pfad steht
in den Audit-Evidence-Daten beziehungsweise JSON-Berichten zur Verfügung und
muss nicht vollständig in der kompakten History-Karte dargestellt werden.

## Sichtbare Einschränkungen

- **Register** wird nur für Template Manager und nur bei Komponenten im Status
  **Approved** angezeigt.
- **Publish** wird nur für Template Manager und nur bei Komponenten im Status
  **Registered** angezeigt.
- Komponenten in anderen Status erscheinen nicht in der Komponentenauswahl.
- Fehlt die erforderliche Rolle, werden die zugehörigen Aktionen nicht angezeigt.
- Vor dem Speichern prüft der Dienst die direkte Abhängigkeit erneut. Wurde die
  ausgewählte Komponente zwischenzeitlich entfernt oder ist sie nicht mehr
  **Registered** oder **Published**, zeigt die Erstellungsseite einen
  Abhängigkeitsfehler. Der unvollständige Entwurf wird nicht gespeichert.
- Eine fehlende oder ungültige DID sowie eine zyklische Referenz werden beim
  Hinzufügen abgelehnt und erscheinen nicht in der Hierarchie.
- Meldet das Audit weiterhin fehlende Data Requirements oder Klausel-Bindings,
  prüfen Sie sowohl den Parent als auch jede unmittelbar eingebettete
  Komponente. Weiter verschachtelte Component-Snapshots zählen bewusst nicht
  zur effektiven Audit-Sicht.
