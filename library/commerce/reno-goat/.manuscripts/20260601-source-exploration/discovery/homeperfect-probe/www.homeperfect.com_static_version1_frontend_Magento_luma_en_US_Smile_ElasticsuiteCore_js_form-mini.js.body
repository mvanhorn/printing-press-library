/**
 * DISCLAIMER
 *
 * Do not edit or add to this file if you wish to upgrade Smile ElasticSuite to newer
 * versions in the future.
 *
 * @category  Smile
 * @package   Smile\ElasticsuiteCore
 * @author    Romain Ruaud <romain.ruaud@smile.fr>
 * @copyright 2020 Smile
 * @license   Open Software License ("OSL") v. 3.0
 */

/*jshint browser:true jquery:true*/
/*global alert*/

define([
    'ko',
    'jquery',
    'underscore',
    'mage/template',
    'Magento_Catalog/js/price-utils',
    'Magento_Ui/js/lib/knockout/template/loader',
    'Magento_Ui/js/modal/modal',
    'mage/translate',
    'Magento_Search/js/form-mini'
], function (ko, $, _, mageTemplate, priceUtil, templateLoader) {
    'use strict';

    $.widget('smileEs.quickSearch', $.mage.quickSearch, {
        options: {
            autocomplete: 'off',
            minSearchLength: 2,
            responseFieldElements: 'dl dd',
            selectClass: 'selected',
            submitBtn: 'button[type="submit"]',
            searchLabel: '[data-role=minisearch-label]'
        },

        /**
         * Overriden constructor to ensure templates initialization on load
         *
         * @private
         */
        _create: function () {
            this.templateCache = [];
            this.currentRequest = null;
            this._initTemplates();
            this._initTitleRenderer();
            this._super();
            this._blur();
        },

        /**
         * Init templates used for rendering when instantiating the widget
         *
         * @private
         */
        _initTemplates: function() {
            for (var template in this.options.templates) {
                if ({}.hasOwnProperty.call(this.options.templates, template)) {
                    this._loadTemplate(template);
                }
            }
        },

        /**
         * Init templates used for rendering when instantiating the widget
         *
         * @private
         */
        _initTitleRenderer: function() {
            this.titleRenderers = {};
            for (var typeIdentifier in this.options.templates) {
                if (this.options.templates[typeIdentifier]['titleRenderer']) {
                   require([this.options.templates[typeIdentifier]['titleRenderer']], function (renderer) {
                       this.component.titleRenderers[this.type] = renderer;
                   }.bind({component: this, type: typeIdentifier}));
                }
            }
        },

        /**
         * Load a renderer for title when configured for a type.
         *
         * @param type The type to render
         *
         * @private
         */
        _loadTemplate: function (type) {
            var templateFile = this.options.templates[type]['template'];
            templateLoader.loadTemplate(templateFile).done(function (renderer) {
                this.options.templates[type]['template'] = renderer;
            }.bind(this));
        },

        /**
         * Get rendering template for a given element. Will look into this.options.templates[element.type] for the renderer.
         * Returns an evaluated template for the given element's type.
         *
         * @param element The autocomplete result to display
         *
         * @returns function
         *
         * @private
         */
        _getTemplate: function (element) {
            var source = this.options.template; // Fallback to standard widget template
            var type   = element.type ? element.type : 'undefined';

            if (this.templateCache[type]) {
                return this.templateCache[type];
            }

            if (element.type && this.options.templates && this.options.templates[element.type]) {
                source = this.options.templates[element.type].template;
            }

            this.templateCache[type] = mageTemplate(source);

            return this.templateCache[type];
        },

        /**
         * Render an autocomplete item in the result list
         *
         * @param element The element : an autocomplete result
         * @param index   The element index
         *
         * @returns {*|jQuery|HTMLElement}
         *
         * @private
         */
        _renderItem: function (element, index) {
            var template = this._getTemplate(element);
            element.index = index;

            if (element.price && (!isNaN(element.price))) {
                element.price = priceUtil.formatPrice(element.price, this.options.priceFormat);
            }

            return template({
                data: element
            });
        },

        /**
         * Return the wrapper for all autocomplete results
         *
         * @returns {*|jQuery|HTMLElement}
         *
         * @private
         */
        _getResultWrapper: function () {
            return $('<div class="smile-elasticsuite-autocomplete-result"></div>');
        },

        /**
         * Return the header for an autocomplete result section
         *
         * @param type The type of element to display
         *
         * @returns {*|jQuery|HTMLElement}
         *
         * @private
         */
        _getSectionHeader: function(type, data) {
            var title = '';
            var header = $('<dl role="listbox" class="autocomplete-list"></dl>');

            if (type !== undefined) {
                title = this._getSectionTitle(type, data);
                header.append(title);
            }

            return header;
        },

        /**
         * Return the title for an autocomplete result section
         *
         * @param type
         *
         * @returns {*|jQuery|HTMLElement}
         *
         * @private
         */
        _getSectionTitle: function(type, data) {
            var title = '';

            if (this.titleRenderers && this.titleRenderers[type]) {
                title = $('<dt role="listbox" class="autocomplete-list-title title-' + type + '">' + this.titleRenderers[type].render(data) + '</dt>');
            } else if (this.options.templates && this.options.templates[type].title) {
                title = $('<dt role="listbox" class="autocomplete-list-title title-' + type + '">' + this.options.templates[type].title + '</dt>');
            }

            return title;
        },

        /**
         * Check wether the incoming string is not empty or if doesn't consist of spaces.
         *
         * @param {String} value - Value to check.
         *
         * @returns {Boolean}
         *
         * @private
         */
        _isEmpty : function(value) {
            return value === null || value.trim().length === 0;
        },

        /**
         * Executes when the value of the search input field changes. Executes a GET request
         * to populate a suggestion list based on entered text. Handles click (select), hover,
         * and mouseout events on the populated suggestion list dropdown.
         *
         * Overriden to :
         *  - move rendering of elements in a subfunction.
         *  - manage redirection when clicking a result having an href attribute.
         *
         * @private
         */
        _onPropertyChange: _.debounce(function () {
            var searchField = this.element,
                clonePosition = {
                    position: 'absolute',
                    // Removed to fix display issues
                    // left: searchField.offset().left,
                    // top: searchField.offset().top + searchField.outerHeight(),
                    width: searchField.outerWidth()
                },
                value = this.element.val();

            this.submitBtn.disabled = this._isEmpty(value);

            if (value.trim().length >= parseInt(this.options.minSearchLength, 10)) {
                this.searchForm.addClass('processing');
                this.currentRequest = $.ajax({
                    method: "GET",
                    url: this.options.url,
                    cache: true,
                    dataType: 'json',
                    data: {q: value},
                    // This function will ensure proper killing of the last Ajax call.
                    // In order to prevent requests of an old request to pop up later and replace results.
                    beforeSend: function() { if (this.currentRequest !== null) { this.currentRequest.abort(); }}.bind(this),
                    success: $.proxy(function (data) {
                        var self = this;
                        var lastElement = false;
                        var content = this._getResultWrapper();
                        var sectionDropdown = this._getSectionHeader();
                        $.each(data, function(index, element) {

                            if (!lastElement || (lastElement && lastElement.type !== element.type)) {
                                sectionDropdown = this._getSectionHeader(element.type, data);
                            }

                            var elementHtml = this._renderItem(element, index);

                            sectionDropdown.append(elementHtml);

                            if (!lastElement || (lastElement && lastElement.type !== element.type)) {
                                content.append(sectionDropdown);
                            }

                            lastElement = element;
                        }.bind(this));
                        this.responseList.indexList = this.autoComplete.html(content)
                            .css(clonePosition)
                            .show()
                            .find(this.options.responseFieldElements + ':visible');

                        this._resetResponseList(false);
                        this.element.removeAttr('aria-activedescendant');

                        if (this.responseList.indexList.length) {
                            this._updateAriaHasPopup(true);
                        } else {
                            this._updateAriaHasPopup(false);
                        }

                        this.responseList.indexList
                            .on('click vclick', function (e) {
                                self.responseList.selected = $(this);
                                if (self.responseList.selected.attr("href")) {
                                    window.location.href = self.responseList.selected.attr("href");
                                    e.stopPropagation();
                                    return false;
                                }
                                self.searchForm.trigger('submit');
                            })
                            .on('mouseenter', function (e) {
                                self.responseList.indexList.removeClass(self.options.selectClass);
                                $(this).addClass(self.options.selectClass);
                                self.responseList.selected = $(e.target);
                                self.element.attr('aria-activedescendant', $(e.target).attr('id'));
                            })
                            .on('mouseleave', function (e) {
                                $(this).removeClass(self.options.selectClass);
                                self._resetResponseList(false);
                            })
                            .on('mouseout', function () {
                                if (!self._getLastElement() && self._getLastElement().hasClass(self.options.selectClass)) {
                                    $(this).removeClass(self.options.selectClass);
                                    self._resetResponseList(false);
                                }
                            });
                    },this),
                    complete : $.proxy(function () {
                        this.searchForm.removeClass('processing');
                    }, this)
                });
            } else {
                this._resetResponseList(true);
                this.autoComplete.hide();
                this._updateAriaHasPopup(false);
                this.element.removeAttr('aria-activedescendant');
            }
        }, 250),

        /**
         * Executes when keys are pressed in the search input field. Performs specific actions
         * depending on which keys are pressed.
         *
         * @private
         * @param {Event} e - The key down event
         * @return {Boolean} Default return type for any unhandled keys
         */
        _onKeyDown: function (e) {
            var keyCode = e.keyCode || e.which;

            switch (keyCode) {
                case $.ui.keyCode.HOME:
                    this._selectElement(this._getFirstVisibleElement());
                    break;
                case $.ui.keyCode.END:
                    this._selectElement(this._getLastElement());
                    break;
                case $.ui.keyCode.ESCAPE:
                    this._resetResponseList(true);
                    this.autoComplete.hide();
                    break;
                case $.ui.keyCode.ENTER:
                    this._validateElement(e);
                    break;
                case $.ui.keyCode.DOWN:
                    this._navigateDown();
                    break;
                case $.ui.keyCode.UP:
                    this._navigateUp();
                    break;
                default:
                    return true;
            }
        },

        /**
         * Validate selection of an element (eg : when ENTER is pressed)
         *
         * @param event The keydown event
         *
         * @returns {boolean}
         *
         * @private
         */
        _validateElement: function(event) {
            var selected = this.responseList.selected;
            if (selected && selected.attr('href') !== undefined) {
                window.location = selected.attr('href');
                event.preventDefault();
                return false;
            }
            this.searchForm.trigger('submit');
        },

        /**
         * Process down navigation on autocomplete box
         *
         * @private
         */
        _navigateDown: function() {
            if (this.responseList.indexList) {
                if (!this.responseList.selected) {
                    this._getFirstVisibleElement().addClass(this.options.selectClass);
                    this.responseList.selected = this._getFirstVisibleElement();
                }
                else if (!this._getLastElement().hasClass(this.options.selectClass)) {
                    var nextElement = this._getNextElement();
                    this.responseList.selected.removeClass(this.options.selectClass);
                    this.responseList.selected = nextElement.addClass(this.options.selectClass);
                } else {
                    this.responseList.selected.removeClass(this.options.selectClass);
                    this._getFirstVisibleElement().addClass(this.options.selectClass);
                    this.responseList.selected = this._getFirstVisibleElement();
                }
                this._activateElement();
            }
        },

        /**
         * Process up navigation on autocomplete box
         *
         * @private
         */
        _navigateUp: function() {
            if (this.responseList.indexList !== null) {
                if (!this._getFirstVisibleElement().hasClass(this.options.selectClass)) {
                    var prevElement = this._getPrevElement();
                    this.responseList.selected.removeClass(this.options.selectClass);
                    this.responseList.selected = prevElement.addClass(this.options.selectClass);
                } else {
                    this.responseList.selected.removeClass(this.options.selectClass);
                    this._getLastElement().addClass(this.options.selectClass);
                    this.responseList.selected = this._getLastElement();
                }
                this._activateElement();
            }
        },

        /**
         * Toggles an element as currently selected
         *
         * @param {Element} e - The DOM element
         *
         * @private
         */
        _selectElement: function(element) {
            element.addClass(this.options.selectClass);
            this.responseList.selected = element;
        },

        /**
         * Toggles an element as active
         *
         * @param {Element} e - The DOM element
         *
         * @private
         */
        _activateElement: function() {
            this.element.val(this.responseList.selected.find('.qs-option-name').text());
            this.element.attr('aria-activedescendant', this.responseList.selected.attr('id'));
        },

        /**
         * Retrieve the next element when navigating through keyboard
         *
         * @private
         *
         * @return Element
         */
        _getNextElement: function() {
            var nextElement = this.responseList.selected.next('dd');
            if (nextElement.length === 0) {
                nextElement = this.responseList.selected.parent('dl').next('dl').find('dd').first();
            }

            return nextElement;
        },

        /**
         * Retrieve the previous element when navigating through keyboard
         *
         * @private
         *
         * @return Element
         */
        _getPrevElement: function() {
            var prevElement = this.responseList.selected.prev('dd');
            this.responseList.selected.removeClass(this.options.selectClass);
            if (prevElement.length === 0) {
                prevElement = this.responseList.selected.parent('dl').prev('dl').find('dd').last();
            }

            return prevElement;
        },

        /**
         * Handle blur event of search input item
         * @private
         */
        _blur: function() {
            this.element.on('blur', $.proxy(function () {
                if (!this.searchLabel.hasClass('active')) {
                    return;
                }
                setTimeout($.proxy(function () {
                    if (this.autoComplete.is(':hidden')) {
                        this.setActiveState(false);
                    } else {
                        this.element.trigger('focus');
                    }
                    this.autoComplete.hide();
                    $('#search').trigger('blur');
                    this._updateAriaHasPopup(false);
                }, this),250);
            }, this));
        }
    });

    return $.smileEs.quickSearch;
});
